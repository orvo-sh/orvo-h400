package traceservice

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetTrace(ctx context.Context, orgID string, traceID string) (*GetTraceOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetTrace: fetching trace",
		slog.String("organization_id", orgID),
		slog.String("trace_id", traceID),
	)

	query := `WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.trace_id,
			t.span_id,
			t.parent_span_id,
			t.trace_state,
			t.name,
			t.kind,
			t.start_time,
			t.end_time,
			t.duration_ns,
			t.status_code,
			t.status_message,
			t.resource_attributes,
			t.scope_attributes,
			t.span_attributes,
			t.resource_schema_url,
			t.scope_name,
			t.scope_version,
			t.scope_schema_url,
			t.events,
			t.links,
			t.service_name,
			t.deployment_environment
		FROM traces_hot t
		WHERE t.organization_id = $1
		  AND t.trace_id = $2

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.trace_id,
			r.span_id,
			r.parent_span_id,
			r.trace_state,
			r.name,
			r.kind,
			r.start_time,
			r.end_time,
			r.duration_ns,
			r.status_code,
			r.status_message,
			r.resource_attributes,
			r.scope_attributes,
			r.span_attributes,
			r.resource_schema_url,
			r.scope_name,
			r.scope_version,
			r.scope_schema_url,
			r.events,
			r.links,
			r.service_name,
			r.deployment_environment
		FROM traces_restored r
		WHERE r.organization_id = $1
		  AND r.trace_id = $2
		  AND NOT EXISTS (
			SELECT 1
			FROM traces_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
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
	FROM unioned
	ORDER BY start_time ASC`

	rows, err := s.pg.Pool().Query(ctx, query, orgID, traceID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetTrace: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var spans []models.Span
	for rows.Next() {
		var sp models.Span
		var resourceAttrsRaw []byte
		var scopeAttrsRaw []byte
		var spanAttrsRaw []byte
		var eventsRaw []byte
		var linksRaw []byte

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
			&resourceAttrsRaw,
			&scopeAttrsRaw,
			&spanAttrsRaw,
			&sp.ResourceSchemaURL,
			&sp.ScopeName,
			&sp.ScopeVersion,
			&sp.ScopeSchemaURL,
			&eventsRaw,
			&linksRaw,
			&sp.ServiceName,
			&sp.DeploymentEnvironment,
		); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		if err := parseJSONMap(resourceAttrsRaw, &sp.ResourceAttributes); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: parse resource attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseJSONMap(scopeAttrsRaw, &sp.ScopeAttributes); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: parse scope attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseJSONMap(spanAttrsRaw, &sp.SpanAttributes); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: parse span attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseSpanEvents(eventsRaw, &sp.Events); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: parse events failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseSpanLinks(linksRaw, &sp.Links); err != nil {
			s.logger.ErrorContext(ctx, "GetTrace: parse links failed", slog.Any("error", err))
			return nil, errs.ErrInternal
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

func parseJSONMap(raw []byte, out *map[string]string) error {
	if len(raw) == 0 {
		*out = map[string]string{}
		return nil
	}
	var value map[string]string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value == nil {
		value = map[string]string{}
	}
	*out = value
	return nil
}

func parseSpanEvents(raw []byte, out *[]models.SpanEvent) error {
	if len(raw) == 0 {
		*out = []models.SpanEvent{}
		return nil
	}
	var value []models.SpanEvent
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value == nil {
		value = []models.SpanEvent{}
	}
	*out = value
	return nil
}

func parseSpanLinks(raw []byte, out *[]models.SpanLink) error {
	if len(raw) == 0 {
		*out = []models.SpanLink{}
		return nil
	}
	var value []models.SpanLink
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if value == nil {
		value = []models.SpanLink{}
	}
	*out = value
	return nil
}
