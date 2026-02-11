package sink

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/pkg/batcher"
)

type LogSink struct {
	clickhouse *clickhouse.DB
	batcher    *batcher.Batcher[models.LogRecord]
}

func NewLogSink(clickhouse *clickhouse.DB, logger *slog.Logger) *LogSink {
	s := &LogSink{
		clickhouse: clickhouse,
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

	batch, err := s.clickhouse.PrepareBatch(ctx, `INSERT INTO logs (
		id,
        timestamp, observed_timestamp,
        severity_number, severity_text, body,
        trace_id, span_id, trace_flags,
        resource_attributes, resource_schema_url,
        scope_name, scope_version, scope_attributes, scope_schema_url,
        log_attributes,
        service_name, deployment_environment,
        organization_id
    )`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, r := range records {
		if err := batch.Append(
			r.ID,
			r.Timestamp, r.ObservedTimestamp,
			r.SeverityNumber, r.SeverityText, r.Body,
			r.TraceID, r.SpanID, r.TraceFlags,
			r.ResourceAttributes, r.ResourceSchemaURL,
			r.ScopeName, r.ScopeVersion, r.ScopeAttributes, r.ScopeSchemaURL,
			r.LogAttributes,
			r.ServiceName, r.DeploymentEnvironment,
			r.OrganizationID,
		); err != nil {
			return fmt.Errorf("failed to append record to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}
