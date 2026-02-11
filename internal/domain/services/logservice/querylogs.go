package logservice

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
	defaultLimit = 100
	maxLimit     = 1000
)

// fieldMapping maps user-facing filter field names to ClickHouse column expressions.
func fieldMapping(field string) (string, bool) {
	switch field {
	case "service", "service_name":
		return "service_name", true
	case "level", "severity", "severity_text":
		return "severity_text", true
	case "severity_number":
		return "severity_number", true
	case "body", "message":
		return "body", true
	case "trace_id":
		return "trace_id", true
	case "span_id":
		return "span_id", true
	case "environment", "deployment_environment":
		return "deployment_environment", true
	default:
		return "", false
	}
}

// attributeExpression returns a ClickHouse map lookup expression for attribute fields.
// Fields prefixed with "resource.", "scope.", or "log." (or "attr.") are looked up in
// the corresponding attributes map.
func attributeExpression(field string) (string, bool) {
	switch {
	case strings.HasPrefix(field, "resource."):
		key := strings.TrimPrefix(field, "resource.")
		return fmt.Sprintf("resource_attributes['%s']", key), true
	case strings.HasPrefix(field, "scope."):
		key := strings.TrimPrefix(field, "scope.")
		return fmt.Sprintf("scope_attributes['%s']", key), true
	case strings.HasPrefix(field, "log."), strings.HasPrefix(field, "attr."):
		key := field
		if strings.HasPrefix(field, "log.") {
			key = strings.TrimPrefix(field, "log.")
		} else {
			key = strings.TrimPrefix(field, "attr.")
		}
		return fmt.Sprintf("log_attributes['%s']", key), true
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

func (s *service) QueryLogs(ctx context.Context, input QueryLogsInput) (*QueryLogsOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryLogs: querying logs", slog.Any("input", input))

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
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, input.StartTime)
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, input.EndTime)
	}

	// Cursor-based pagination: cursor is a timestamp; fetch logs older than cursor
	if input.Cursor != nil {
		clauses = append(clauses, "timestamp < ?")
		args = append(args, *input.Cursor)
	}

	// Full-text search on body
	if input.SearchQuery != "" {
		clauses = append(clauses, "body ILIKE ?")
		args = append(args, "%"+input.SearchQuery+"%")
	}

	// Dynamic filters
	for _, f := range input.Filters {
		col, ok := resolveField(f.Field)
		if !ok {
			s.logger.WarnContext(ctx, "QueryLogs: unknown filter field, skipping", slog.String("field", f.Field))
			continue
		}

		clause, filterArgs, err := buildFilterClause(col, f)
		if err != nil {
			s.logger.WarnContext(ctx, "QueryLogs: invalid filter, skipping",
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

	query := fmt.Sprintf(`SELECT
		id,
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
		deployment_environment,
		organization_id
	FROM logs
	WHERE %s
	ORDER BY timestamp DESC
	LIMIT ?`, where)

	args = append(args, limit)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryLogs: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var logs []models.LogRecord
	for rows.Next() {
		var rec models.LogRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.Timestamp,
			&rec.ObservedTimestamp,
			&rec.SeverityNumber,
			&rec.SeverityText,
			&rec.Body,
			&rec.TraceID,
			&rec.SpanID,
			&rec.TraceFlags,
			&rec.ResourceAttributes,
			&rec.ResourceSchemaURL,
			&rec.ScopeName,
			&rec.ScopeVersion,
			&rec.ScopeAttributes,
			&rec.ScopeSchemaURL,
			&rec.LogAttributes,
			&rec.ServiceName,
			&rec.DeploymentEnvironment,
			&rec.OrganizationID,
		); err != nil {
			s.logger.ErrorContext(ctx, "QueryLogs: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		logs = append(logs, rec)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "QueryLogs: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	// Determine next cursor
	var nextCursor *time.Time
	if len(logs) == limit {
		last := logs[len(logs)-1].Timestamp
		nextCursor = &last
	}

	return &QueryLogsOutput{
		Logs:       logs,
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

// splitValues splits a comma-separated value string for IN/NIN operators.
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
