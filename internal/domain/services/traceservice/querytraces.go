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

// fieldMapping maps user-facing filter field names to ClickHouse column expressions.
func fieldMapping(field string) (string, bool) {
	switch field {
	case "service", "service_name":
		return "service_name", true
	case "name", "span_name":
		return "name", true
	case "kind", "span_kind":
		return "kind", true
	case "status", "status_code":
		return "status_code", true
	case "environment", "deployment_environment":
		return "deployment_environment", true
	case "trace_id":
		return "trace_id", true
	case "span_id":
		return "span_id", true
	case "parent_span_id":
		return "parent_span_id", true
	default:
		return "", false
	}
}

// attributeExpression returns a ClickHouse map lookup expression for attribute fields.
func attributeExpression(field string) (string, bool) {
	switch {
	case strings.HasPrefix(field, "resource."):
		key := strings.TrimPrefix(field, "resource.")
		return fmt.Sprintf("resource_attributes['%s']", key), true
	case strings.HasPrefix(field, "scope."):
		key := strings.TrimPrefix(field, "scope.")
		return fmt.Sprintf("scope_attributes['%s']", key), true
	case strings.HasPrefix(field, "span."), strings.HasPrefix(field, "attr."):
		key := field
		if strings.HasPrefix(field, "span.") {
			key = strings.TrimPrefix(field, "span.")
		} else {
			key = strings.TrimPrefix(field, "attr.")
		}
		return fmt.Sprintf("span_attributes['%s']", key), true
	default:
		return "", false
	}
}

func resolveField(field string) (string, bool) {
	if col, ok := fieldMapping(field); ok {
		return col, true
	}
	return attributeExpression(field)
}

func (s *service) QueryTraces(ctx context.Context, input QueryTracesInput) (*QueryTracesOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryTraces: querying traces", slog.Any("input", input))

	limit := input.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	var (
		clauses []string
		args    []any
	)

	// Organization scoping (always required)
	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	// Time range
	if !input.StartTime.IsZero() {
		clauses = append(clauses, "start_time >= ?")
		args = append(args, input.StartTime)
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, "start_time <= ?")
		args = append(args, input.EndTime)
	}

	// Cursor-based pagination
	if input.Cursor != nil {
		clauses = append(clauses, "start_time < ?")
		args = append(args, *input.Cursor)
	}

	// Search on span name
	if input.SearchQuery != "" {
		clauses = append(clauses, "name ILIKE ?")
		args = append(args, "%"+input.SearchQuery+"%")
	}

	// Dynamic filters
	for _, f := range input.Filters {
		col, ok := resolveField(f.Field)
		if !ok {
			s.logger.WarnContext(ctx, "QueryTraces: unknown filter field, skipping", slog.String("field", f.Field))
			continue
		}

		clause, filterArgs, err := buildFilterClause(col, f)
		if err != nil {
			s.logger.WarnContext(ctx, "QueryTraces: invalid filter, skipping",
				slog.String("field", f.Field),
				slog.String("operator", string(f.Operator)),
				slog.Any("error", err),
			)
			continue
		}
		clauses = append(clauses, clause)
		args = append(args, filterArgs...)
	}

	where := strings.Join(clauses, " AND ")

	// Duration filters are applied on the aggregated result (HAVING clause)
	var havingClauses []string
	var havingArgs []any

	if input.MinDurationMs > 0 {
		havingClauses = append(havingClauses, "trace_duration_ns >= ?")
		havingArgs = append(havingArgs, input.MinDurationMs*1_000_000)
	}
	if input.MaxDurationMs > 0 {
		havingClauses = append(havingClauses, "trace_duration_ns <= ?")
		havingArgs = append(havingArgs, input.MaxDurationMs*1_000_000)
	}

	havingSQL := ""
	if len(havingClauses) > 0 {
		havingSQL = "HAVING " + strings.Join(havingClauses, " AND ")
	}

	// Query trace summaries by grouping spans by trace_id.
	// We find the root span (parent_span_id = '' or all zeros) to get the root name/service.
	query := fmt.Sprintf(`SELECT
		trace_id,
		argMin(name, if(parent_span_id = '' OR parent_span_id = '0000000000000000', 0, 1)) AS root_span_name,
		argMin(service_name, if(parent_span_id = '' OR parent_span_id = '0000000000000000', 0, 1)) AS root_service,
		min(start_time) AS trace_start,
		toInt64(max(end_time) - min(start_time)) AS trace_duration_ns,
		count() AS span_count,
		countIf(status_code = 2) AS error_count
	FROM spans
	WHERE %s
	GROUP BY trace_id
	%s
	ORDER BY trace_start DESC
	LIMIT ?`, where, havingSQL)

	allArgs := append(args, havingArgs...)
	allArgs = append(allArgs, limit)

	rows, err := s.ch.Query(ctx, query, allArgs...)
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

	// Determine next cursor
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

// buildFilterClause converts a single Filter into a SQL clause with positional args.
func buildFilterClause(col string, f Filter) (string, []any, error) {
	switch f.Operator {
	case FilterOperatorContains:
		return fmt.Sprintf("%s ILIKE ?", col), []any{"%" + f.Value + "%"}, nil
	case FilterOperatorNotContains:
		return fmt.Sprintf("%s NOT ILIKE ?", col), []any{"%" + f.Value + "%"}, nil
	case FilterOperatorIn:
		vals := splitValues(f.Value)
		placeholders := make([]string, len(vals))
		args := make([]any, len(vals))
		for i, v := range vals {
			placeholders[i] = "?"
			args[i] = v
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ",")), args, nil
	case FilterOperatorNotIn:
		vals := splitValues(f.Value)
		placeholders := make([]string, len(vals))
		args := make([]any, len(vals))
		for i, v := range vals {
			placeholders[i] = "?"
			args[i] = v
		}
		return fmt.Sprintf("%s NOT IN (%s)", col, strings.Join(placeholders, ",")), args, nil
	case FilterOperatorGt:
		return fmt.Sprintf("%s > ?", col), []any{f.Value}, nil
	case FilterOperatorGte:
		return fmt.Sprintf("%s >= ?", col), []any{f.Value}, nil
	case FilterOperatorLt:
		return fmt.Sprintf("%s < ?", col), []any{f.Value}, nil
	case FilterOperatorLte:
		return fmt.Sprintf("%s <= ?", col), []any{f.Value}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", f.Operator)
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
