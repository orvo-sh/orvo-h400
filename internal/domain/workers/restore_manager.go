package workers

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

type RestoreManagerConfig struct {
	RestoredTTL          time.Duration
	RestoreThroughputBPS int64
	MaxJobsPerPass       int
}

type RestoreManager struct {
	logger           *slog.Logger
	pool             *pgxpool.Pool
	store            ObjectStore
	partitionManager *PartitionManager
	config           RestoreManagerConfig
	trigger          chan struct{}
}

type CreateRestoreJobInput struct {
	OrganizationID string
	Signal         TelemetrySignal
	StartDay       time.Time
	EndDay         time.Time
	RequestedBy    string
}

type restoreJobClaim struct {
	ID             string
	OrganizationID string
	Signal         TelemetrySignal
}

type restoreJobItem struct {
	ID              string
	Day             time.Time
	ObjectKey       string
	ObjectSizeBytes int64
}

func NewRestoreManager(
	logger *slog.Logger,
	pool *pgxpool.Pool,
	store ObjectStore,
	partitionManager *PartitionManager,
	config RestoreManagerConfig,
) *RestoreManager {
	if config.RestoredTTL <= 0 {
		config.RestoredTTL = 24 * time.Hour
	}
	if config.MaxJobsPerPass <= 0 {
		config.MaxJobsPerPass = 4
	}

	return &RestoreManager{
		logger:           logger.With("module", "restore_manager"),
		pool:             pool,
		store:            store,
		partitionManager: partitionManager,
		config:           config,
		trigger:          make(chan struct{}, 1),
	}
}

func (m *RestoreManager) Enabled() bool {
	return m.store != nil && m.store.Enabled()
}

func (m *RestoreManager) TriggerChan() <-chan struct{} {
	return m.trigger
}

func (m *RestoreManager) Notify() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

func (m *RestoreManager) CreateJob(ctx context.Context, input CreateRestoreJobInput) (*models.RestoreJob, apperr.Error) {
	if !m.Enabled() {
		return nil, errs.ErrArchiveNotConfigured
	}

	spec, ok := getSignalSpec(input.Signal)
	if !ok {
		return nil, errs.ErrBadRequest
	}

	if input.OrganizationID == "" {
		return nil, errs.ErrBadRequest
	}

	startDay := truncateUTCDate(input.StartDay)
	endDay := truncateUTCDate(input.EndDay)
	if endDay.Before(startDay) {
		startDay, endDay = endDay, startDay
	}

	objects, err := m.listRestorableObjects(ctx, input.OrganizationID, spec.Signal, startDay, endDay)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to list restorable objects", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	if len(objects) == 0 {
		return nil, errs.ErrArchiveObjectMissing
	}

	totalBytes := int64(0)
	for _, object := range objects {
		totalBytes += object.ObjectSizeBytes
	}

	estimatedSeconds := 0
	if m.config.RestoreThroughputBPS > 0 && totalBytes > 0 {
		estimatedSeconds = int((totalBytes + m.config.RestoreThroughputBPS - 1) / m.config.RestoreThroughputBPS)
	}

	jobID := util.GenerateID("rsj")
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to begin restore transaction", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback(context.Background())
		}
	}()

	if _, err := tx.Exec(ctx, `
		INSERT INTO restore_jobs (
			id,
			organization_id,
			signal,
			start_day,
			end_day,
			state,
			requested_by,
			total_items,
			completed_items,
			total_bytes,
			done_bytes,
			estimated_seconds,
			error,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,'queued',$6,$7,0,$8,0,$9,'',NOW(),NOW()
		)
	`, jobID, input.OrganizationID, string(spec.Signal), startDay, endDay, input.RequestedBy, len(objects), totalBytes, estimatedSeconds); err != nil {
		m.logger.ErrorContext(ctx, "failed to create restore job", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	for _, object := range objects {
		itemID := util.GenerateID("rsi")
		if _, err := tx.Exec(ctx, `
			INSERT INTO restore_job_items (
				id,
				restore_job_id,
				organization_id,
				signal,
				day,
				object_key,
				state,
				object_size_bytes,
				restored_rows,
				error,
				created_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,'queued',$7,0,'',NOW()
			)
		`, itemID, jobID, input.OrganizationID, string(spec.Signal), object.Day, object.ObjectKey, object.ObjectSizeBytes); err != nil {
			m.logger.ErrorContext(ctx, "failed to create restore job item", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
	}

	if err := tx.Commit(ctx); err != nil {
		m.logger.ErrorContext(ctx, "failed to commit restore transaction", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	rollback = false

	job, appErr := m.GetJob(ctx, input.OrganizationID, jobID)
	if appErr != nil {
		return nil, appErr
	}

	m.Notify()
	return job, nil
}

func (m *RestoreManager) GetJob(ctx context.Context, organizationID string, jobID string) (*models.RestoreJob, apperr.Error) {
	if organizationID == "" || jobID == "" {
		return nil, errs.ErrBadRequest
	}

	row := m.pool.QueryRow(ctx, `
		SELECT
			id,
			organization_id,
			signal,
			start_day,
			end_day,
			state,
			COALESCE(requested_by, ''),
			total_items,
			completed_items,
			total_bytes,
			done_bytes,
			estimated_seconds,
			error,
			created_at,
			started_at,
			finished_at
		FROM restore_jobs
		WHERE organization_id = $1
		  AND id = $2
	`, organizationID, jobID)

	var job models.RestoreJob
	if err := row.Scan(
		&job.ID,
		&job.OrganizationID,
		&job.Signal,
		&job.StartDay,
		&job.EndDay,
		&job.State,
		&job.RequestedBy,
		&job.TotalItems,
		&job.CompletedItems,
		&job.TotalBytes,
		&job.DoneBytes,
		&job.EstimatedSeconds,
		&job.Error,
		&job.CreatedAt,
		&job.StartedAt,
		&job.FinishedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		m.logger.ErrorContext(ctx, "failed to load restore job", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	items, err := m.listJobItems(ctx, job.ID)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to list restore job items", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	job.Items = items

	return &job, nil
}

func (m *RestoreManager) ProcessQueued(ctx context.Context) error {
	if !m.Enabled() {
		return nil
	}

	for i := 0; i < m.config.MaxJobsPerPass; i++ {
		job, err := m.claimNextJob(ctx)
		if err != nil {
			return err
		}
		if job == nil {
			return nil
		}
		if err := m.processJob(ctx, *job); err != nil {
			return err
		}
	}

	return nil
}

func (m *RestoreManager) listRestorableObjects(
	ctx context.Context,
	organizationID string,
	signal TelemetrySignal,
	startDay time.Time,
	endDay time.Time,
) ([]restoreJobItem, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT o.day, o.object_key, o.object_size_bytes
		FROM (
			SELECT DISTINCT ON (day)
				day,
				object_key,
				object_size_bytes,
				created_at
			FROM archive_objects
			WHERE organization_id = $1
			  AND signal = $2
			  AND day >= $3
			  AND day <= $4
			  AND deleted_at IS NULL
			ORDER BY day ASC, created_at DESC
		) o
		WHERE NOT EXISTS (
			SELECT 1
			FROM restored_coverage rc
			WHERE rc.organization_id = $1
			  AND rc.signal = $2
			  AND rc.day = o.day
			  AND rc.expires_at > NOW()
		)
		ORDER BY o.day ASC
	`, organizationID, string(signal), startDay, endDay)
	if err != nil {
		return nil, fmt.Errorf("query restorable archive objects: %w", err)
	}
	defer rows.Close()

	objects := make([]restoreJobItem, 0)
	for rows.Next() {
		var item restoreJobItem
		if err := rows.Scan(&item.Day, &item.ObjectKey, &item.ObjectSizeBytes); err != nil {
			return nil, fmt.Errorf("scan restorable archive object: %w", err)
		}
		item.Day = truncateUTCDate(item.Day)
		objects = append(objects, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate restorable archive objects: %w", err)
	}

	return objects, nil
}

func (m *RestoreManager) listJobItems(ctx context.Context, jobID string) ([]models.RestoreJobItem, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT
			id,
			day,
			object_key,
			state,
			object_size_bytes,
			restored_rows,
			error,
			started_at,
			finished_at
		FROM restore_job_items
		WHERE restore_job_id = $1
		ORDER BY day ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.RestoreJobItem, 0)
	for rows.Next() {
		var item models.RestoreJobItem
		if err := rows.Scan(
			&item.ID,
			&item.Day,
			&item.ObjectKey,
			&item.State,
			&item.ObjectSizeBytes,
			&item.RestoredRows,
			&item.Error,
			&item.StartedAt,
			&item.FinishedAt,
		); err != nil {
			return nil, err
		}
		item.Day = truncateUTCDate(item.Day)
		items = append(items, item)
	}

	return items, rows.Err()
}

func (m *RestoreManager) claimNextJob(ctx context.Context) (*restoreJobClaim, error) {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin restore claim transaction: %w", err)
	}
	defer tx.Rollback(context.Background())

	var claim restoreJobClaim
	row := tx.QueryRow(ctx, `
		SELECT id, organization_id, signal
		FROM restore_jobs
		WHERE state = 'queued'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`)
	if err := row.Scan(&claim.ID, &claim.OrganizationID, &claim.Signal); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan queued restore job: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE restore_jobs
		SET state = 'running',
			started_at = COALESCE(started_at, NOW()),
			updated_at = NOW()
		WHERE id = $1
	`, claim.ID); err != nil {
		return nil, fmt.Errorf("mark restore job running: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit restore claim transaction: %w", err)
	}

	return &claim, nil
}

func (m *RestoreManager) processJob(ctx context.Context, job restoreJobClaim) error {
	spec, ok := getSignalSpec(job.Signal)
	if !ok {
		return m.failJob(ctx, job.ID, fmt.Errorf("unknown signal %q", job.Signal))
	}

	rows, err := m.pool.Query(ctx, `
		SELECT
			id,
			day,
			object_key,
			object_size_bytes
		FROM restore_job_items
		WHERE restore_job_id = $1
		  AND state = 'queued'
		ORDER BY day ASC
	`, job.ID)
	if err != nil {
		return m.failJob(ctx, job.ID, fmt.Errorf("list restore job items: %w", err))
	}
	defer rows.Close()

	items := make([]restoreJobItem, 0)
	for rows.Next() {
		var item restoreJobItem
		if err := rows.Scan(&item.ID, &item.Day, &item.ObjectKey, &item.ObjectSizeBytes); err != nil {
			return m.failJob(ctx, job.ID, fmt.Errorf("scan restore job item: %w", err))
		}
		item.Day = truncateUTCDate(item.Day)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return m.failJob(ctx, job.ID, fmt.Errorf("iterate restore job items: %w", err))
	}

	for _, item := range items {
		if err := m.processItem(ctx, job, spec, item); err != nil {
			return m.failJob(ctx, job.ID, err)
		}
	}

	if _, err := m.pool.Exec(ctx, `
		UPDATE restore_jobs
		SET state = 'completed',
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, job.ID); err != nil {
		return fmt.Errorf("complete restore job: %w", err)
	}

	return nil
}

func (m *RestoreManager) processItem(ctx context.Context, job restoreJobClaim, spec signalSpec, item restoreJobItem) error {
	if _, err := m.pool.Exec(ctx, `
		UPDATE restore_job_items
		SET state = 'running',
			started_at = COALESCE(started_at, NOW())
		WHERE id = $1
	`, item.ID); err != nil {
		return fmt.Errorf("mark restore item running: %w", err)
	}

	dayStart := truncateUTCDate(item.Day)
	dayEnd := dayStart.AddDate(0, 0, 1)

	if err := m.partitionManager.EnsurePartitionsForDay(ctx, dayStart); err != nil {
		return fmt.Errorf("ensure restore partitions: %w", err)
	}

	deleteQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE organization_id = $1 AND %s >= $2 AND %s < $3",
		spec.RestoreTable,
		spec.TimeColumn,
		spec.TimeColumn,
	)
	if _, err := m.pool.Exec(ctx, deleteQuery, job.OrganizationID, dayStart, dayEnd); err != nil {
		return fmt.Errorf("delete existing restored rows: %w", err)
	}

	objectBody, err := m.store.Get(ctx, item.ObjectKey)
	if err != nil {
		return fmt.Errorf("download restore object: %w", err)
	}
	defer objectBody.Close()

	gzipReader, err := gzip.NewReader(objectBody)
	if err != nil {
		return fmt.Errorf("open restore gzip stream: %w", err)
	}
	defer gzipReader.Close()

	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire postgres connection: %w", err)
	}
	defer conn.Release()

	copySQL := fmt.Sprintf(
		"COPY %s (%s) FROM STDIN WITH (FORMAT csv)",
		spec.RestoreTable,
		strings.Join(spec.Columns, ", "),
	)
	commandTag, err := conn.Conn().PgConn().CopyFrom(ctx, gzipReader, copySQL)
	if err != nil {
		_, _ = m.pool.Exec(context.Background(), `
			UPDATE restore_job_items
			SET state = 'failed',
				error = $2,
				finished_at = NOW()
			WHERE id = $1
		`, item.ID, err.Error())
		return fmt.Errorf("restore copy from archive: %w", err)
	}

	if _, err := m.pool.Exec(ctx, `
		INSERT INTO restored_coverage (
			organization_id,
			signal,
			day,
			restore_job_id,
			expires_at,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,NOW(),NOW()
		)
		ON CONFLICT (organization_id, signal, day)
		DO UPDATE SET
			restore_job_id = EXCLUDED.restore_job_id,
			expires_at = EXCLUDED.expires_at,
			updated_at = NOW()
	`, job.OrganizationID, string(job.Signal), dayStart, job.ID, time.Now().UTC().Add(m.config.RestoredTTL)); err != nil {
		return fmt.Errorf("upsert restored coverage: %w", err)
	}

	if _, err := m.pool.Exec(ctx, `
		UPDATE restore_job_items
		SET state = 'done',
			restored_rows = $2,
			error = '',
			finished_at = NOW()
		WHERE id = $1
	`, item.ID, commandTag.RowsAffected()); err != nil {
		return fmt.Errorf("mark restore item done: %w", err)
	}

	if _, err := m.pool.Exec(ctx, `
		UPDATE restore_jobs
		SET completed_items = completed_items + 1,
			done_bytes = done_bytes + $2,
			updated_at = NOW()
		WHERE id = $1
	`, job.ID, item.ObjectSizeBytes); err != nil {
		return fmt.Errorf("update restore job progress: %w", err)
	}

	return nil
}

func (m *RestoreManager) failJob(ctx context.Context, jobID string, cause error) error {
	_, _ = m.pool.Exec(context.Background(), `
		UPDATE restore_jobs
		SET state = 'failed',
			error = $2,
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, jobID, cause.Error())
	return cause
}
