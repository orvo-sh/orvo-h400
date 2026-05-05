package remediationservice

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/metricservice"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

const (
	autoResolveThresholdMetricName = "derived.errors.rate"
	defaultThresholdLookback       = 5 * time.Minute
	defaultThresholdCooldown       = 15 * time.Minute
	defaultThresholdQuorum         = 3
)

type metricQuerier interface {
	QueryTimeseries(ctx context.Context, input metricservice.QueryTimeseriesInput) (*metricservice.QueryTimeseriesOutput, apperr.Error)
}

type UpsertAutoResolveThresholdInput struct {
	OrganizationID string
	ServiceName    string
	UserID         string
	ThresholdValue float64
	LookbackWindow time.Duration
	Cooldown       time.Duration
	Quorum         int
	Enabled        bool
}

type thresholdEvaluation struct {
	Signal              float64
	SampleCount         int
	BreachSamples       int
	EstimatedErrorCount int
	PeakSignal          float64
	LatestSignal        float64
	Breached            bool
}

func (s *service) ensureAutoResolveThresholdSchema(ctx context.Context) apperr.Error {
	s.thresholdSchemaOnce.Do(func() {
		statements := []string{
			`
			CREATE TABLE IF NOT EXISTS auto_resolve_thresholds (
				id VARCHAR(32) PRIMARY KEY,
				organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
				service_name TEXT NOT NULL,
				metric_name TEXT NOT NULL DEFAULT 'derived.errors.rate',
				threshold_value DOUBLE PRECISION NOT NULL CHECK (threshold_value > 0),
				lookback_window_seconds INT NOT NULL CHECK (lookback_window_seconds > 0),
				cooldown_seconds INT NOT NULL CHECK (cooldown_seconds > 0),
				quorum INT NOT NULL CHECK (quorum > 0),
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				last_triggered_at TIMESTAMPTZ,
				created_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
				updated_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (organization_id, service_name)
			)
			`,
			`
			CREATE INDEX IF NOT EXISTS idx_auto_resolve_thresholds_org_enabled
				ON auto_resolve_thresholds (organization_id, enabled, updated_at DESC)
			`,
			`
			CREATE INDEX IF NOT EXISTS idx_auto_resolve_thresholds_service
				ON auto_resolve_thresholds (organization_id, service_name)
			`,
		}

		for _, statement := range statements {
			if _, err := s.pg.Pool().Exec(ctx, statement); err != nil {
				s.logger.ErrorContext(ctx, "failed to ensure auto resolve threshold schema", "error", err)
				s.thresholdSchemaErr = err
				return
			}
		}
	})

	if s.thresholdSchemaErr != nil {
		return errs.ErrInternal
	}

	return nil
}

func (s *service) ListAutoResolveThresholds(ctx context.Context, organizationID string) ([]models.AutoResolveThreshold, apperr.Error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, errs.ErrBadRequest
	}
	if appErr := s.ensureAutoResolveThresholdSchema(ctx); appErr != nil {
		return nil, appErr
	}

	rows, err := s.pg.Pool().Query(ctx, `
		SELECT
			id,
			organization_id,
			service_name,
			metric_name,
			threshold_value,
			lookback_window_seconds,
			cooldown_seconds,
			quorum,
			enabled,
			last_triggered_at,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		FROM auto_resolve_thresholds
		WHERE organization_id = $1
		ORDER BY service_name ASC
	`, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list auto resolve thresholds", "error", err)
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	thresholds := make([]models.AutoResolveThreshold, 0)
	for rows.Next() {
		threshold, scanErr := scanAutoResolveThreshold(rows)
		if scanErr != nil {
			s.logger.ErrorContext(ctx, "failed to scan auto resolve threshold", "error", scanErr)
			return nil, errs.ErrInternal
		}
		thresholds = append(thresholds, threshold)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "failed to iterate auto resolve thresholds", "error", err)
		return nil, errs.ErrInternal
	}

	return thresholds, nil
}

func (s *service) UpsertAutoResolveThreshold(ctx context.Context, input UpsertAutoResolveThresholdInput) (*models.AutoResolveThreshold, apperr.Error) {
	serviceName := strings.TrimSpace(input.ServiceName)
	if strings.TrimSpace(input.OrganizationID) == "" || serviceName == "" || strings.TrimSpace(input.UserID) == "" {
		return nil, errs.ErrBadRequest
	}
	if input.ThresholdValue <= 0 {
		return nil, errs.ErrBadRequest
	}
	if appErr := s.ensureAutoResolveThresholdSchema(ctx); appErr != nil {
		return nil, appErr
	}

	lookback := input.LookbackWindow
	if lookback <= 0 {
		lookback = defaultThresholdLookback
	}
	cooldown := input.Cooldown
	if cooldown <= 0 {
		cooldown = defaultThresholdCooldown
	}
	quorum := input.Quorum
	if quorum <= 0 {
		quorum = defaultThresholdQuorum
	}
	if input.Enabled {
		if strings.TrimSpace(s.config.OpencodeCommand) == "" || strings.TrimSpace(s.config.OpencodeModel) == "" {
			return nil, errs.ErrAutoResolveOpencodeMissing
		}
		if _, appErr := s.loadMappingForService(ctx, input.OrganizationID, serviceName); appErr != nil {
			return nil, appErr
		}
	}

	row := s.pg.Pool().QueryRow(ctx, `
		INSERT INTO auto_resolve_thresholds (
			id,
			organization_id,
			service_name,
			metric_name,
			threshold_value,
			lookback_window_seconds,
			cooldown_seconds,
			quorum,
			enabled,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW()
		)
		ON CONFLICT (organization_id, service_name)
		DO UPDATE SET
			metric_name = EXCLUDED.metric_name,
			threshold_value = EXCLUDED.threshold_value,
			lookback_window_seconds = EXCLUDED.lookback_window_seconds,
			cooldown_seconds = EXCLUDED.cooldown_seconds,
			quorum = EXCLUDED.quorum,
			enabled = EXCLUDED.enabled,
			updated_by_user_id = EXCLUDED.updated_by_user_id,
			updated_at = NOW()
		RETURNING
			id,
			organization_id,
			service_name,
			metric_name,
			threshold_value,
			lookback_window_seconds,
			cooldown_seconds,
			quorum,
			enabled,
			last_triggered_at,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
	`,
		util.GenerateID("art"),
		input.OrganizationID,
		serviceName,
		autoResolveThresholdMetricName,
		input.ThresholdValue,
		int(lookback.Seconds()),
		int(cooldown.Seconds()),
		quorum,
		input.Enabled,
		input.UserID,
		input.UserID,
	)

	threshold, err := scanAutoResolveThresholdRow(row)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert auto resolve threshold", "error", err)
		return nil, errs.ErrInternal
	}

	return threshold, nil
}

func (s *service) DeleteAutoResolveThreshold(ctx context.Context, organizationID string, serviceName string) apperr.Error {
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(serviceName) == "" {
		return errs.ErrBadRequest
	}
	if appErr := s.ensureAutoResolveThresholdSchema(ctx); appErr != nil {
		return appErr
	}

	if _, err := s.pg.Pool().Exec(ctx, `
		DELETE FROM auto_resolve_thresholds
		WHERE organization_id = $1
		  AND service_name = $2
	`, organizationID, strings.TrimSpace(serviceName)); err != nil {
		s.logger.ErrorContext(ctx, "failed to delete auto resolve threshold", "error", err)
		return errs.ErrInternal
	}

	return nil
}

func (s *service) ProcessAutoResolveThresholds(ctx context.Context) error {
	if s.metricService == nil {
		return nil
	}
	if appErr := s.ensureAutoResolveThresholdSchema(ctx); appErr != nil {
		return fmt.Errorf("ensure auto resolve threshold schema: %w", appErr)
	}

	rows, err := s.pg.Pool().Query(ctx, `
		SELECT
			id,
			organization_id,
			service_name,
			metric_name,
			threshold_value,
			lookback_window_seconds,
			cooldown_seconds,
			quorum,
			enabled,
			last_triggered_at,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		FROM auto_resolve_thresholds
		WHERE enabled = TRUE
		ORDER BY updated_at ASC
	`)
	if err != nil {
		return fmt.Errorf("list auto resolve thresholds: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		threshold, scanErr := scanAutoResolveThreshold(rows)
		if scanErr != nil {
			return fmt.Errorf("scan auto resolve threshold: %w", scanErr)
		}
		if err := s.processAutoResolveThreshold(ctx, threshold); err != nil {
			s.logger.ErrorContext(ctx, "failed to process auto resolve threshold",
				slog.String("threshold_id", threshold.ID),
				slog.String("organization_id", threshold.OrganizationID),
				slog.String("service_name", threshold.ServiceName),
				slog.Any("error", err),
			)
		}
	}

	return rows.Err()
}

func (s *service) processAutoResolveThreshold(ctx context.Context, threshold models.AutoResolveThreshold) error {
	lookback := time.Duration(threshold.LookbackWindowSeconds) * time.Second
	if lookback <= 0 {
		lookback = defaultThresholdLookback
	}
	cooldown := time.Duration(threshold.CooldownSeconds) * time.Second
	if cooldown <= 0 {
		cooldown = defaultThresholdCooldown
	}

	if threshold.LastTriggeredAt != nil && time.Since(*threshold.LastTriggeredAt) < cooldown {
		return nil
	}

	if active, err := s.hasActiveAutoResolveJob(ctx, threshold.OrganizationID, threshold.ServiceName); err == nil && active {
		return nil
	}

	now := time.Now().UTC()
	seriesOut, appErr := s.metricService.QueryTimeseries(ctx, metricservice.QueryTimeseriesInput{
		OrganizationID: threshold.OrganizationID,
		MetricName:     autoResolveThresholdMetricName,
		StartTime:      now.Add(-lookback),
		EndTime:        now,
		Step:           autoResolveThresholdStep(lookback),
		Aggregation:    "rate",
		ServiceName:    threshold.ServiceName,
	})
	if appErr != nil {
		return fmt.Errorf("query threshold timeseries: %w", appErr)
	}
	if seriesOut == nil || len(seriesOut.Series) == 0 {
		return nil
	}

	step := autoResolveThresholdStep(lookback)
	evaluation := evaluateThresholdSeries(
		seriesOut.Series[0].Points,
		threshold.ThresholdValue,
		threshold.Quorum,
		autoResolveThresholdStepSeconds(step),
	)
	if !evaluation.Breached {
		return nil
	}

	logRecord, err := s.loadLatestErrorLogForService(ctx, threshold.OrganizationID, threshold.ServiceName, now.Add(-lookback))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return fmt.Errorf("load latest error log: %w", err)
	}

	job, appErr := s.RunAutoResolve(ctx, RunAutoResolveInput{
		OrganizationID: threshold.OrganizationID,
		LogID:          logRecord.ID,
		UserID:         threshold.CreatedByUserID,
	})
	if appErr != nil {
		return fmt.Errorf("run auto resolve: %w", appErr)
	}

	if _, err := s.pg.Pool().Exec(ctx, `
		UPDATE auto_resolve_thresholds
		SET last_triggered_at = NOW()
		WHERE id = $1
	`, threshold.ID); err != nil {
		return fmt.Errorf("mark threshold triggered: %w", err)
	}

	s.logger.InfoContext(ctx, "auto resolve threshold triggered",
		slog.String("threshold_id", threshold.ID),
		slog.String("organization_id", threshold.OrganizationID),
		slog.String("service_name", threshold.ServiceName),
		slog.Float64("threshold_value", threshold.ThresholdValue),
		slog.String("log_id", logRecord.ID),
		slog.String("job_id", job.ID),
	)

	return nil
}

func (s *service) hasActiveAutoResolveJob(ctx context.Context, organizationID string, serviceName string) (bool, error) {
	var active bool
	err := s.pg.Pool().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM sandbox_jobs j
			WHERE j.organization_id = $1
			  AND j.mode = 'auto_resolve'
			  AND j.state IN ('queued', 'running')
			  AND coalesce(j.incident_context_json::jsonb -> 'log' ->> 'service_name', '') = $2
		)
	`, organizationID, serviceName).Scan(&active)
	return active, err
}

func autoResolveThresholdStep(lookback time.Duration) string {
	switch {
	case lookback <= 30*time.Second:
		return "10s"
	case lookback <= 2*time.Minute:
		return "30s"
	case lookback <= 15*time.Minute:
		return "1m"
	case lookback <= time.Hour:
		return "5m"
	default:
		return "15m"
	}
}

func autoResolveThresholdStepSeconds(step string) int {
	switch step {
	case "10s":
		return 10
	case "30s":
		return 30
	case "1m":
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	default:
		return 60
	}
}

func evaluateThresholdSeries(
	points []models.TimeseriesPoint,
	thresholdValue float64,
	quorum int,
	bucketSeconds int,
) thresholdEvaluation {
	if quorum <= 0 {
		quorum = defaultThresholdQuorum
	}
	if len(points) == 0 {
		return thresholdEvaluation{}
	}
	if bucketSeconds <= 0 {
		bucketSeconds = 60
	}

	var sum float64
	var peak float64
	breachSamples := 0
	for _, point := range points {
		sum += point.Value
		if point.Value > peak {
			peak = point.Value
		}
		if point.Value >= thresholdValue {
			breachSamples++
		}
	}

	estimatedErrors := int(math.Round(sum * float64(bucketSeconds)))
	latestSignal := points[len(points)-1].Value

	return thresholdEvaluation{
		Signal:              sum / float64(len(points)),
		SampleCount:         len(points),
		BreachSamples:       breachSamples,
		EstimatedErrorCount: estimatedErrors,
		PeakSignal:          peak,
		LatestSignal:        latestSignal,
		Breached:            estimatedErrors >= quorum && peak >= thresholdValue,
	}
}

type thresholdScanner interface {
	Scan(dest ...any) error
}

func scanAutoResolveThresholdRow(row thresholdScanner) (*models.AutoResolveThreshold, error) {
	threshold, err := scanAutoResolveThresholdInto(row)
	if err != nil {
		return nil, err
	}
	return &threshold, nil
}

func scanAutoResolveThreshold(row thresholdScanner) (models.AutoResolveThreshold, error) {
	return scanAutoResolveThresholdInto(row)
}

func scanAutoResolveThresholdInto(row thresholdScanner) (models.AutoResolveThreshold, error) {
	var threshold models.AutoResolveThreshold
	var lastTriggeredAt sql.NullTime
	err := row.Scan(
		&threshold.ID,
		&threshold.OrganizationID,
		&threshold.ServiceName,
		&threshold.MetricName,
		&threshold.ThresholdValue,
		&threshold.LookbackWindowSeconds,
		&threshold.CooldownSeconds,
		&threshold.Quorum,
		&threshold.Enabled,
		&lastTriggeredAt,
		&threshold.CreatedByUserID,
		&threshold.UpdatedByUserID,
		&threshold.CreatedAt,
		&threshold.UpdatedAt,
	)
	if err != nil {
		return models.AutoResolveThreshold{}, err
	}
	if lastTriggeredAt.Valid {
		timestamp := lastTriggeredAt.Time
		threshold.LastTriggeredAt = &timestamp
	}
	return threshold, nil
}
