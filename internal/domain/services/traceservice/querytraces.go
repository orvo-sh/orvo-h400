package traceservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type sqlArgBuilder struct {
	args []any
}

func (b *sqlArgBuilder) add(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

// fieldMapping maps user-facing filter field names to SQL column expressions.
func fieldMapping(alias string, field string) (string, bool) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}

	switch field {
	case "service", "service_name":
		return prefix + "service_name", true
	case "name", "span_name":
		return prefix + "name", true
	case "kind", "span_kind":
		return prefix + "kind", true
	case "status", "status_code":
		return prefix + "status_code", true
	case "environment", "deployment_environment":
		return prefix + "deployment_environment", true
	case "trace_id":
		return prefix + "trace_id", true
	case "span_id":
		return prefix + "span_id", true
	case "parent_span_id":
		return prefix + "parent_span_id", true
	default:
		return "", false
	}
}

// attributeExpression returns a JSON attribute lookup expression for attribute fields.
func attributeExpression(alias string, field string) (string, bool) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}

	switch {
	case strings.HasPrefix(field, "resource."):
		key := strings.TrimPrefix(field, "resource.")
		return fmt.Sprintf("%sresource_attributes->>'%s'", prefix, key), true
	case strings.HasPrefix(field, "scope."):
		key := strings.TrimPrefix(field, "scope.")
		return fmt.Sprintf("%sscope_attributes->>'%s'", prefix, key), true
	case strings.HasPrefix(field, "span."), strings.HasPrefix(field, "attr."):
		key := field
		if strings.HasPrefix(field, "span.") {
			key = strings.TrimPrefix(field, "span.")
		} else {
			key = strings.TrimPrefix(field, "attr.")
		}
		return fmt.Sprintf("%sspan_attributes->>'%s'", prefix, key), true
	default:
		return "", false
	}
}

func resolveField(alias string, field string) (string, bool) {
	if col, ok := fieldMapping(alias, field); ok {
		return col, true
	}
	return attributeExpression(alias, field)
}

func (s *service) QueryTraces(ctx context.Context, input QueryTracesInput) (*QueryTracesOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryTraces: querying traces", slog.Any("input", input))

	if restoreErr := s.checkRestoreRequired(ctx, input); restoreErr != nil {
		return nil, restoreErr
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	args := &sqlArgBuilder{}
	hotWhere := s.buildWhereClause(ctx, "t", input, args)
	restoredWhere := s.buildWhereClause(ctx, "r", input, args)

	postClauses := make([]string, 0, 3)
	if input.Cursor != nil {
		postClauses = append(postClauses, fmt.Sprintf("trace_start < %s", args.add(*input.Cursor)))
	}
	if input.MinDurationMs > 0 {
		postClauses = append(postClauses, fmt.Sprintf("trace_duration_ns >= %s", args.add(input.MinDurationMs*1_000_000)))
	}
	if input.MaxDurationMs > 0 {
		postClauses = append(postClauses, fmt.Sprintf("trace_duration_ns <= %s", args.add(input.MaxDurationMs*1_000_000)))
	}

	postWhere := ""
	if len(postClauses) > 0 {
		postWhere = "WHERE " + strings.Join(postClauses, " AND ")
	}

	limitArg := args.add(limit)

	query := fmt.Sprintf(`WITH spans_union AS (
		SELECT
			t.organization_id,
			t.trace_id,
			t.parent_span_id,
			t.name,
			t.service_name,
			t.start_time,
			t.end_time,
			t.status_code
		FROM traces_hot t
		WHERE %s

		UNION ALL

		SELECT
			r.organization_id,
			r.trace_id,
			r.parent_span_id,
			r.name,
			r.service_name,
			r.start_time,
			r.end_time,
			r.status_code
		FROM traces_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM traces_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	),
	grouped AS (
		SELECT
			trace_id,
			(array_agg(name ORDER BY CASE WHEN parent_span_id = '' OR parent_span_id = '0000000000000000' THEN 0 ELSE 1 END, start_time ASC))[1] AS root_span_name,
			(array_agg(service_name ORDER BY CASE WHEN parent_span_id = '' OR parent_span_id = '0000000000000000' THEN 0 ELSE 1 END, start_time ASC))[1] AS root_service,
			min(start_time) AS trace_start,
			(extract(epoch FROM (max(end_time) - min(start_time))) * 1000000000)::BIGINT AS trace_duration_ns,
			count(*)::BIGINT AS span_count,
			count(*) FILTER (WHERE status_code = 2)::BIGINT AS error_count
		FROM spans_union
		GROUP BY trace_id
	)
	SELECT
		trace_id,
		root_span_name,
		root_service,
		trace_start,
		trace_duration_ns,
		span_count,
		error_count
	FROM grouped
	%s
	ORDER BY trace_start DESC
	LIMIT %s`, hotWhere, restoredWhere, postWhere, limitArg)

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryTraces: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var traces []models.TraceSummary
	for rows.Next() {
		var t models.TraceSummary
		if err := rows.Scan(
			&t.TraceID,
			&t.RootSpanName,
			&t.RootService,
			&t.StartTime,
			&t.DurationNs,
			&t.SpanCount,
			&t.ErrorCount,
		); err != nil {
			s.logger.ErrorContext(ctx, "QueryTraces: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		traces = append(traces, t)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "QueryTraces: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	var nextCursor *time.Time
	if len(traces) == limit {
		last := traces[len(traces)-1].StartTime
		nextCursor = &last
	}

	return &QueryTracesOutput{
		Traces:     traces,
		NextCursor: nextCursor,
	}, nil
}

func (s *service) buildWhereClause(ctx context.Context, alias string, input QueryTracesInput, args *sqlArgBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
	}

	if !input.StartTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%sstart_time >= %s", prefix, args.add(input.StartTime)))
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%sstart_time <= %s", prefix, args.add(input.EndTime)))
	}
	if input.SearchQuery != "" {
		clauses = append(clauses, fmt.Sprintf("%sname ILIKE %s", prefix, args.add("%"+input.SearchQuery+"%")))
	}

	for _, f := range input.Filters {
		col, ok := resolveField(alias, f.Field)
		if !ok {
			s.logger.WarnContext(ctx, "QueryTraces: unknown filter field, skipping", slog.String("field", f.Field))
			continue
		}

		clause, err := buildFilterClause(col, f, args)
		if err != nil {
			s.logger.WarnContext(ctx, "QueryTraces: invalid filter, skipping",
				slog.String("field", f.Field),
				slog.String("operator", string(f.Operator)),
				slog.Any("error", err),
			)
			continue
		}
		clauses = append(clauses, clause)
	}

	return strings.Join(clauses, " AND ")
}

// buildFilterClause converts a single Filter into a SQL clause.
func buildFilterClause(col string, f Filter, args *sqlArgBuilder) (string, error) {
	switch f.Operator {
	case FilterOperatorContains:
		return fmt.Sprintf("%s ILIKE %s", col, args.add("%"+f.Value+"%")), nil
	case FilterOperatorNotContains:
		return fmt.Sprintf("%s NOT ILIKE %s", col, args.add("%"+f.Value+"%")), nil
	case FilterOperatorIn:
		vals := splitValues(f.Value)
		if len(vals) == 0 {
			return "", fmt.Errorf("empty IN filter values")
		}
		placeholders := make([]string, 0, len(vals))
		for _, v := range vals {
			placeholders = append(placeholders, args.add(v))
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ",")), nil
	case FilterOperatorNotIn:
		vals := splitValues(f.Value)
		if len(vals) == 0 {
			return "", fmt.Errorf("empty NOT IN filter values")
		}
		placeholders := make([]string, 0, len(vals))
		for _, v := range vals {
			placeholders = append(placeholders, args.add(v))
		}
		return fmt.Sprintf("%s NOT IN (%s)", col, strings.Join(placeholders, ",")), nil
	case FilterOperatorGt:
		return fmt.Sprintf("%s > %s", col, args.add(f.Value)), nil
	case FilterOperatorGte:
		return fmt.Sprintf("%s >= %s", col, args.add(f.Value)), nil
	case FilterOperatorLt:
		return fmt.Sprintf("%s < %s", col, args.add(f.Value)), nil
	case FilterOperatorLte:
		return fmt.Sprintf("%s <= %s", col, args.add(f.Value)), nil
	default:
		return "", fmt.Errorf("unsupported operator: %s", f.Operator)
	}
}

func splitValues(v string) []string {
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
