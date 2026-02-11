package traceservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetTrace(ctx context.Context, orgID string, traceID string) (*GetTraceOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetTrace: fetching trace",
		slog.String("organization_id", orgID),
		slog.String("trace_id", traceID),
	)

	query := `SELECT
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
		events_name,
		events_timestamp,
		events_attributes,
		links_trace_id,
		links_span_id,
		links_trace_state,
		links_attributes,
		service_name,
		deployment_environment
	FROM spans
	WHERE organization_id = ?
	  AND trace_id = ?
	ORDER BY start_time ASC`

	rows, err := s.ch.Query(ctx, query, orgID, traceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetTrace: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var spans []models.Span
	for rows.Next() {
		var sp models.Span
		var eventsName []string
		var eventsTimestamp []time.Time
		var eventsAttributes []map[string]string
		var linksTraceID []string
		var linksSpanID []string
		var linksTraceState []string
		var linksAttributes []map[string]string

		if err := rows.Scan(
			&sp.ID,
			&sp.OrganizationID,
			&sp.TraceID,
			&sp.SpanID,
			&sp.ParentSpanID,
			&sp.TraceState,
			&sp.Name,
			&sp.Kind,
			&sp.StartTime,
			&sp.EndTime,
			&sp.DurationNs,
			&sp.StatusCode,
			&sp.StatusMessage,
			&sp.ResourceAttributes,
			&sp.ScopeAttributes,
			&sp.SpanAttributes,
			&sp.ResourceSchemaURL,
			&sp.ScopeName,
			&sp.ScopeVersion,
			&sp.ScopeSchemaURL,
			&eventsName,
			&eventsTimestamp,
			&eventsAttributes,
			&linksTraceID,
			&linksSpanID,
			&linksTraceState,
			&linksAttributes,
			&sp.ServiceName,
			&sp.DeploymentEnvironment,
		); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		// Reconstruct events from parallel arrays.
		sp.Events = make([]models.SpanEvent, len(eventsName))
		for i := range eventsName {
			sp.Events[i] = models.SpanEvent{
				Name:      eventsName[i],
				Timestamp: eventsTimestamp[i],
			}
			if i < len(eventsAttributes) {
				sp.Events[i].Attributes = eventsAttributes[i]
			}
		}

		// Reconstruct links from parallel arrays.
		sp.Links = make([]models.SpanLink, len(linksTraceID))
		for i := range linksTraceID {
			sp.Links[i] = models.SpanLink{
				TraceID: linksTraceID[i],
				SpanID:  linksSpanID[i],
			}
			if i < len(linksTraceState) {
				sp.Links[i].TraceState = linksTraceState[i]
			}
			if i < len(linksAttributes) {
				sp.Links[i].Attributes = linksAttributes[i]
			}
		}

		spans = append(spans, sp)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetTrace: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &GetTraceOutput{
		Spans: spans,
	}, nil
}
