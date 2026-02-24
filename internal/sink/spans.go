package sink

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/batcher"
)

type SpanSink struct {
	postgres *postgres.DB
	batcher  *batcher.Batcher[models.Span]
}

func NewSpanSink(postgres *postgres.DB, logger *slog.Logger) *SpanSink {
	s := &SpanSink{
		postgres: postgres,
	}
	s.batcher = batcher.New(
		logger.With("module", "SpanSinkBatcher"),
		s.writeBatch,
		batcher.WithBatchSize(10000),
		batcher.WithFlushInterval(5*time.Second),
		batcher.WithMaxQueueSize(100000),
	)
	return s
}

func (s *SpanSink) Enqueue(_ context.Context, records []models.Span) error {
	for _, r := range records {
		if err := s.batcher.Push(r); err != nil {
			return fmt.Errorf("failed to enqueue span: %w", err)
		}
	}
	return nil
}

func (s *SpanSink) Close() error {
	s.batcher.Close()
	return nil
}

func (s *SpanSink) writeBatch(ctx context.Context, records []models.Span) error {
	if len(records) == 0 {
		return nil
	}

	const query = `INSERT INTO traces_hot (
		id,
		organization_id,
		trace_id,
		span_id,
		parent_span_id,
		trace_state,
		name,
		kind,
		start_time,
		end_time,
		duration_ns,
		status_code,
		status_message,
		resource_attributes,
		scope_attributes,
		span_attributes,
		resource_schema_url,
		scope_name,
		scope_version,
		scope_schema_url,
		events,
		links,
		service_name,
		deployment_environment
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,
		$14::jsonb,$15::jsonb,$16::jsonb,$17,$18,$19,$20,$21::jsonb,$22::jsonb,$23,$24
	)`

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
		spanAttrs, err := jsonObject(r.SpanAttributes)
		if err != nil {
			return fmt.Errorf("marshal span attributes: %w", err)
		}
		events, err := jsonArray(r.Events)
		if err != nil {
			return fmt.Errorf("marshal span events: %w", err)
		}
		links, err := jsonArray(r.Links)
		if err != nil {
			return fmt.Errorf("marshal span links: %w", err)
		}

		dbBatch.Queue(
			query,
			r.ID,
			r.OrganizationID,
			r.TraceID,
			r.SpanID,
			r.ParentSpanID,
			r.TraceState,
			r.Name,
			r.Kind,
			r.StartTime,
			r.EndTime,
			r.DurationNs,
			r.StatusCode,
			r.StatusMessage,
			resourceAttrs,
			scopeAttrs,
			spanAttrs,
			r.ResourceSchemaURL,
			r.ScopeName,
			r.ScopeVersion,
			r.ScopeSchemaURL,
			events,
			links,
			r.ServiceName,
			r.DeploymentEnvironment,
		)
	}

	results := s.postgres.Pool().SendBatch(ctx, &dbBatch)
	for range records {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert traces_hot batch: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close traces_hot batch: %w", err)
	}

	return nil
}
