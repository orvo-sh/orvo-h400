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

type SpanSink struct {
	clickhouse *clickhouse.DB
	batcher    *batcher.Batcher[models.Span]
}

func NewSpanSink(clickhouse *clickhouse.DB, logger *slog.Logger) *SpanSink {
	s := &SpanSink{
		clickhouse: clickhouse,
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

	batch, err := s.clickhouse.PrepareBatch(ctx, `INSERT INTO spans (
		id,
		organization_id,
		trace_id, span_id, parent_span_id, trace_state,
		name, kind,
		start_time, end_time, duration_ns,
		status_code, status_message,
		resource_attributes, scope_attributes, span_attributes,
		resource_schema_url, scope_name, scope_version, scope_schema_url,
		events_name, events_timestamp, events_attributes,
		links_trace_id, links_span_id, links_trace_state, links_attributes,
		service_name, deployment_environment
	)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, r := range records {
		// Convert events to parallel arrays.
		eventsName := make([]string, len(r.Events))
		eventsTimestamp := make([]time.Time, len(r.Events))
		eventsAttributes := make([]map[string]string, len(r.Events))
		for i, e := range r.Events {
			eventsName[i] = e.Name
			eventsTimestamp[i] = e.Timestamp
			eventsAttributes[i] = e.Attributes
			if eventsAttributes[i] == nil {
				eventsAttributes[i] = map[string]string{}
			}
		}

		// Convert links to parallel arrays.
		linksTraceID := make([]string, len(r.Links))
		linksSpanID := make([]string, len(r.Links))
		linksTraceState := make([]string, len(r.Links))
		linksAttributes := make([]map[string]string, len(r.Links))
		for i, l := range r.Links {
			linksTraceID[i] = l.TraceID
			linksSpanID[i] = l.SpanID
			linksTraceState[i] = l.TraceState
			linksAttributes[i] = l.Attributes
			if linksAttributes[i] == nil {
				linksAttributes[i] = map[string]string{}
			}
		}

		if err := batch.Append(
			r.ID,
			r.OrganizationID,
			r.TraceID, r.SpanID, r.ParentSpanID, r.TraceState,
			r.Name, r.Kind,
			r.StartTime, r.EndTime, r.DurationNs,
			r.StatusCode, r.StatusMessage,
			r.ResourceAttributes, r.ScopeAttributes, r.SpanAttributes,
			r.ResourceSchemaURL, r.ScopeName, r.ScopeVersion, r.ScopeSchemaURL,
			eventsName, eventsTimestamp, eventsAttributes,
			linksTraceID, linksSpanID, linksTraceState, linksAttributes,
			r.ServiceName, r.DeploymentEnvironment,
		); err != nil {
			return fmt.Errorf("failed to append span to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}
