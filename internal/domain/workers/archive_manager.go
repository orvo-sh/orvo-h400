package workers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/smithy-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/orvo-sh/orvo/pkg/util"
)

type ArchiveManagerConfig struct {
	Prefix                  string
	HotRetentionLogs        time.Duration
	HotRetentionTraces      time.Duration
	HotRetentionMetrics     time.Duration
	ArchiveRetentionLogs    time.Duration
	ArchiveRetentionTraces  time.Duration
	ArchiveRetentionMetrics time.Duration
	MaxObjectsPerRun        int
}

type ArchiveManager struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
	store  ObjectStore
	config ArchiveManagerConfig
}

type archiveCandidate struct {
	OrganizationID string
	Day            time.Time
}

func NewArchiveManager(logger *slog.Logger, pool *pgxpool.Pool, store ObjectStore, config ArchiveManagerConfig) *ArchiveManager {
	if config.MaxObjectsPerRun <= 0 {
		config.MaxObjectsPerRun = 256
	}

	return &ArchiveManager{
		logger: logger.With("module", "archive_manager"),
		pool:   pool,
		store:  store,
		config: config,
	}
}

func (m *ArchiveManager) Enabled() bool {
	return m.store != nil && m.store.Enabled()
}

func (m *ArchiveManager) ExportDue(ctx context.Context) error {
	if !m.Enabled() {
		m.logger.DebugContext(ctx, "archive export skipped: object store disabled")
		return nil
	}

	signals := []TelemetrySignal{SignalLogs, SignalTraces, SignalMetrics}
	for _, signal := range signals {
		if err := m.exportSignal(ctx, signal); err != nil {
			return err
		}
	}

	return nil
}

func (m *ArchiveManager) ApplyRetention(ctx context.Context) error {
	if !m.Enabled() {
		m.logger.DebugContext(ctx, "archive retention skipped: object store disabled")
		return nil
	}

	rows, err := m.pool.Query(ctx, `
		SELECT id, object_key
		FROM archive_objects
		WHERE deleted_at IS NULL
		  AND expires_at <= NOW()
		ORDER BY expires_at ASC
		LIMIT $1
	`, m.config.MaxObjectsPerRun)
	if err != nil {
		return fmt.Errorf("list expired archive objects: %w", err)
	}
	defer rows.Close()

	type expiredObject struct {
		ID  string
		Key string
	}

	objects := make([]expiredObject, 0)
	for rows.Next() {
		var object expiredObject
		if err := rows.Scan(&object.ID, &object.Key); err != nil {
			return fmt.Errorf("scan expired archive object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate expired archive objects: %w", err)
	}

	for _, object := range objects {
		if err := m.store.Delete(ctx, object.Key); err != nil {
			var apiErr smithy.APIError
			if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NoSuchKey" {
				return err
			}
		}

		if _, err := m.pool.Exec(ctx, `
			UPDATE archive_objects
			SET deleted_at = NOW(),
				updated_at = NOW()
			WHERE id = $1
		`, object.ID); err != nil {
			return fmt.Errorf("mark archive object deleted: %w", err)
		}
	}

	return nil
}

func (m *ArchiveManager) exportSignal(ctx context.Context, signal TelemetrySignal) error {
	spec, ok := getSignalSpec(signal)
	if !ok {
		return fmt.Errorf("unknown signal %q", signal)
	}

	cutoff, ok := m.archiveCutoff(signal)
	if !ok {
		return nil
	}

	candidates, err := m.listArchiveCandidates(ctx, spec, cutoff)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	jobID := util.GenerateID("arj")
	if _, err := m.pool.Exec(ctx, `
		INSERT INTO archive_jobs (id, signal, state, started_at, created_at, updated_at)
		VALUES ($1, $2, 'running', NOW(), NOW(), NOW())
	`, jobID, string(signal)); err != nil {
		return fmt.Errorf("create archive job: %w", err)
	}

	markFailed := func(runErr error) error {
		_, _ = m.pool.Exec(context.Background(), `
			UPDATE archive_jobs
			SET state = 'failed',
				error = $2,
				finished_at = NOW(),
				updated_at = NOW()
			WHERE id = $1
		`, jobID, runErr.Error())
		return runErr
	}

	for _, candidate := range candidates {
		if err := m.exportCandidate(ctx, jobID, spec, candidate); err != nil {
			return markFailed(err)
		}
	}

	if _, err := m.pool.Exec(ctx, `
		UPDATE archive_jobs
		SET state = 'completed',
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, jobID); err != nil {
		return fmt.Errorf("complete archive job: %w", err)
	}

	return nil
}

func (m *ArchiveManager) archiveCutoff(signal TelemetrySignal) (time.Time, bool) {
	var retention time.Duration
	switch signal {
	case SignalLogs:
		retention = m.config.HotRetentionLogs
	case SignalTraces:
		retention = m.config.HotRetentionTraces
	case SignalMetrics:
		retention = m.config.HotRetentionMetrics
	}

	if retention <= 0 {
		return time.Time{}, false
	}

	return truncateUTCDate(time.Now().UTC().Add(-retention)), true
}

func (m *ArchiveManager) archiveObjectExpiry(signal TelemetrySignal) time.Time {
	now := time.Now().UTC()
	switch signal {
	case SignalLogs:
		if m.config.ArchiveRetentionLogs > 0 {
			return now.Add(m.config.ArchiveRetentionLogs)
		}
	case SignalTraces:
		if m.config.ArchiveRetentionTraces > 0 {
			return now.Add(m.config.ArchiveRetentionTraces)
		}
	case SignalMetrics:
		if m.config.ArchiveRetentionMetrics > 0 {
			return now.Add(m.config.ArchiveRetentionMetrics)
		}
	}
	return now.Add(365 * 24 * time.Hour)
}

func (m *ArchiveManager) listArchiveCandidates(ctx context.Context, spec signalSpec, cutoff time.Time) ([]archiveCandidate, error) {
	query := fmt.Sprintf(`
		SELECT q.organization_id, q.day
		FROM (
			SELECT
				organization_id,
				date_trunc('day', %s)::date AS day
			FROM %s
			WHERE %s < $1
			GROUP BY organization_id, day
		) AS q
		WHERE NOT EXISTS (
			SELECT 1
			FROM archive_objects ao
			WHERE ao.organization_id = q.organization_id
			  AND ao.signal = $2
			  AND ao.day = q.day
			  AND ao.deleted_at IS NULL
		)
		ORDER BY q.day ASC, q.organization_id ASC
		LIMIT $3
	`, spec.TimeColumn, spec.HotTable, spec.TimeColumn)

	rows, err := m.pool.Query(ctx, query, cutoff, string(spec.Signal), m.config.MaxObjectsPerRun)
	if err != nil {
		return nil, fmt.Errorf("list archive candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]archiveCandidate, 0)
	for rows.Next() {
		var c archiveCandidate
		if err := rows.Scan(&c.OrganizationID, &c.Day); err != nil {
			return nil, fmt.Errorf("scan archive candidate: %w", err)
		}
		c.Day = truncateUTCDate(c.Day)
		candidates = append(candidates, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate archive candidates: %w", err)
	}

	return candidates, nil
}

func (m *ArchiveManager) exportCandidate(ctx context.Context, jobID string, spec signalSpec, candidate archiveCandidate) error {
	payload, rowCount, err := m.copyHotRowsToCompressedCSV(ctx, spec, candidate.OrganizationID, candidate.Day)
	if err != nil {
		return err
	}
	if rowCount == 0 {
		return nil
	}

	checksumRaw := sha256.Sum256(payload)
	checksum := hex.EncodeToString(checksumRaw[:])
	objectKey := m.buildObjectKey(spec.Signal, candidate.OrganizationID, candidate.Day, checksum)

	if err := m.store.Put(ctx, objectKey, bytes.NewReader(payload), int64(len(payload)), "application/gzip"); err != nil {
		return err
	}

	archiveObjectID := util.GenerateID("aobj")
	expiresAt := m.archiveObjectExpiry(spec.Signal)
	if _, err := m.pool.Exec(ctx, `
		INSERT INTO archive_objects (
			id,
			archive_job_id,
			organization_id,
			signal,
			day,
			bucket,
			object_key,
			object_size_bytes,
			row_count,
			checksum,
			schema_version,
			expires_at,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,$11,NOW(),NOW()
		)
	`, archiveObjectID, jobID, candidate.OrganizationID, string(spec.Signal), candidate.Day, m.store.Bucket(), objectKey, len(payload), rowCount, checksum, expiresAt); err != nil {
		return fmt.Errorf("insert archive object metadata: %w", err)
	}

	m.logger.InfoContext(ctx, "archived telemetry partition",
		slog.String("signal", string(spec.Signal)),
		slog.String("organization_id", candidate.OrganizationID),
		slog.String("day", candidate.Day.Format("2006-01-02")),
		slog.String("object_key", objectKey),
		slog.Int64("row_count", rowCount),
		slog.Int("object_size_bytes", len(payload)),
	)

	return nil
}

func (m *ArchiveManager) copyHotRowsToCompressedCSV(ctx context.Context, spec signalSpec, organizationID string, day time.Time) ([]byte, int64, error) {
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("acquire postgres connection: %w", err)
	}
	defer conn.Release()

	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)

	columns := strings.Join(spec.Columns, ", ")
	dayLiteral := sqlDateLiteral(day)
	copySQL := fmt.Sprintf(`
		COPY (
			SELECT %s
			FROM %s
			WHERE organization_id = %s
			  AND %s >= %s::date
			  AND %s < (%s::date + INTERVAL '1 day')
			ORDER BY %s ASC
		) TO STDOUT WITH (FORMAT csv)
	`, columns, spec.HotTable, sqlStringLiteral(organizationID), spec.TimeColumn, dayLiteral, spec.TimeColumn, dayLiteral, spec.TimeColumn)

	commandTag, err := conn.Conn().PgConn().CopyTo(ctx, gzipWriter, copySQL)
	closeErr := gzipWriter.Close()
	if err != nil {
		return nil, 0, fmt.Errorf("copy %s day %s to stdout: %w", spec.Signal, day.Format("2006-01-02"), err)
	}
	if closeErr != nil {
		return nil, 0, fmt.Errorf("close gzip stream: %w", closeErr)
	}

	return compressed.Bytes(), commandTag.RowsAffected(), nil
}

func (m *ArchiveManager) buildObjectKey(signal TelemetrySignal, organizationID string, day time.Time, checksum string) string {
	prefix := strings.TrimSpace(m.config.Prefix)
	prefix = strings.Trim(prefix, "/")

	base := fmt.Sprintf("%s/org=%s/day=%s", signal, organizationID, day.Format("2006-01-02"))
	if prefix != "" {
		base = fmt.Sprintf("%s/%s", prefix, base)
	}

	return fmt.Sprintf("%s/%s.csv.gz", base, checksum[:16])
}
