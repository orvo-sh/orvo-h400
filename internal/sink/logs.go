package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/batcher"
)

type LogSink struct {
	postgres *postgres.DB
	batcher  *batcher.Batcher[models.LogRecord]
}

func NewLogSink(postgres *postgres.DB, logger *slog.Logger) *LogSink {
	s := &LogSink{
		postgres: postgres,
	}
	s.batcher = batcher.New(
		logger.With("module", "LogSinkBatcher"),
		s.writeBatch,
		batcher.WithBatchSize(10000),
		batcher.WithFlushInterval(5*time.Second),
		batcher.WithMaxQueueSize(100000),
	)
	return s
}

func (s *LogSink) Enqueue(_ context.Context, records []models.LogRecord) error {
	for _, r := range records {
		if err := s.batcher.Push(r); err != nil {
			return fmt.Errorf("failed to enqueue log: %w", err)
		}
	}
	return nil
}

func (s *LogSink) Close() error {
	s.batcher.Close()
	return nil
}

func (s *LogSink) writeBatch(ctx context.Context, records []models.LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	var dbBatch pgx.Batch
	for _, r := range records {
		resourceAttrs, err := jsonObject(r.ResourceAttributes)
		if err != nil {
			return fmt.Errorf("marshal resource attributes: %w", err)
		}
		scopeAttrs, err := jsonObject(r.ScopeAttributes)
		if err != nil {
			return fmt.Errorf("marshal scope attributes: %w", err)
		}
		logAttrs, err := jsonObject(r.LogAttributes)
		if err != nil {
			return fmt.Errorf("marshal log attributes: %w", err)
		}

		dbBatch.Queue(`INSERT INTO logs_hot (
			id,
			organization_id,
			timestamp,
			observed_timestamp,
			severity_number,
			severity_text,
			body,
			trace_id,
			span_id,
			trace_flags,
			resource_attributes,
			resource_schema_url,
			scope_name,
			scope_version,
			scope_attributes,
			scope_schema_url,
			log_attributes,
			service_name,
			deployment_environment
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11::jsonb,$12,$13,$14,$15::jsonb,$16,$17::jsonb,$18,$19
		)`,
			r.ID,
			r.OrganizationID,
			r.Timestamp,
			r.ObservedTimestamp,
			r.SeverityNumber,
			r.SeverityText,
			r.Body,
			r.TraceID,
			r.SpanID,
			r.TraceFlags,
			resourceAttrs,
			r.ResourceSchemaURL,
			r.ScopeName,
			r.ScopeVersion,
			scopeAttrs,
			r.ScopeSchemaURL,
			logAttrs,
			r.ServiceName,
			r.DeploymentEnvironment,
		)
	}

	results := s.postgres.Pool().SendBatch(ctx, &dbBatch)
	for range records {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert logs_hot batch: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close logs_hot batch: %w", err)
	}

	return nil
}

func jsonObject(m map[string]string) ([]byte, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func jsonArray(v any) ([]byte, error) {
	if v == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(v)
}
