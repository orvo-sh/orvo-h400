package logservice

import (
	"context"
	"encoding/json"
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
	case "level", "severity", "severity_text":
		return prefix + "severity_text", true
	case "severity_number":
		return prefix + "severity_number", true
	case "body", "message":
		return prefix + "body", true
	case "trace_id":
		return prefix + "trace_id", true
	case "span_id":
		return prefix + "span_id", true
	case "environment", "deployment_environment":
		return prefix + "deployment_environment", true
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
	case strings.HasPrefix(field, "log."), strings.HasPrefix(field, "attr."):
		key := field
		if strings.HasPrefix(field, "log.") {
			key = strings.TrimPrefix(field, "log.")
		} else {
			key = strings.TrimPrefix(field, "attr.")
		}
		return fmt.Sprintf("%slog_attributes->>'%s'", prefix, key), true
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

func (s *service) QueryLogs(ctx context.Context, input QueryLogsInput) (*QueryLogsOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryLogs: querying logs", slog.Any("input", input))

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
	hotWhere := s.buildWhereClause(ctx, "l", input, args)
	restoredWhere := s.buildWhereClause(ctx, "r", input, args)
	limitArg := args.add(limit)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			l.id,
			l.timestamp,
			l.observed_timestamp,
			l.severity_number,
			l.severity_text,
			l.body,
			l.trace_id,
			l.span_id,
			l.trace_flags,
			l.resource_attributes,
			l.resource_schema_url,
			l.scope_name,
			l.scope_version,
			l.scope_attributes,
			l.scope_schema_url,
			l.log_attributes,
			l.service_name,
			l.deployment_environment,
			l.organization_id
		FROM logs_hot l
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.timestamp,
			r.observed_timestamp,
			r.severity_number,
			r.severity_text,
			r.body,
			r.trace_id,
			r.span_id,
			r.trace_flags,
			r.resource_attributes,
			r.resource_schema_url,
			r.scope_name,
			r.scope_version,
			r.scope_attributes,
			r.scope_schema_url,
			r.log_attributes,
			r.service_name,
			r.deployment_environment,
			r.organization_id
		FROM logs_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM logs_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
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
	FROM unioned
	ORDER BY timestamp DESC
	LIMIT %s`, hotWhere, restoredWhere, limitArg)

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryLogs: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var logs []models.LogRecord
	for rows.Next() {
		var rec models.LogRecord
		var resourceAttrsRaw []byte
		var scopeAttrsRaw []byte
		var logAttrsRaw []byte

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
			&resourceAttrsRaw,
			&rec.ResourceSchemaURL,
			&rec.ScopeName,
			&rec.ScopeVersion,
			&scopeAttrsRaw,
			&rec.ScopeSchemaURL,
			&logAttrsRaw,
			&rec.ServiceName,
			&rec.DeploymentEnvironment,
			&rec.OrganizationID,
		); err != nil {
			s.logger.ErrorContext(ctx, "QueryLogs: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		if err := parseJSONMap(resourceAttrsRaw, &rec.ResourceAttributes); err != nil {
			s.logger.ErrorContext(ctx, "QueryLogs: parse resource attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseJSONMap(scopeAttrsRaw, &rec.ScopeAttributes); err != nil {
			s.logger.ErrorContext(ctx, "QueryLogs: parse scope attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		if err := parseJSONMap(logAttrsRaw, &rec.LogAttributes); err != nil {
			s.logger.ErrorContext(ctx, "QueryLogs: parse log attributes failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		logs = append(logs, rec)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "QueryLogs: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

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

func (s *service) buildWhereClause(ctx context.Context, alias string, input QueryLogsInput, args *sqlArgBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
	}

	if !input.StartTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%stimestamp >= %s", prefix, args.add(input.StartTime)))
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%stimestamp <= %s", prefix, args.add(input.EndTime)))
	}
	if input.Cursor != nil {
		clauses = append(clauses, fmt.Sprintf("%stimestamp < %s", prefix, args.add(*input.Cursor)))
	}
	if input.SearchQuery != "" {
		clauses = append(clauses, fmt.Sprintf("%sbody ILIKE %s", prefix, args.add("%"+input.SearchQuery+"%")))
	}

	for _, f := range input.Filters {
		col, ok := resolveField(alias, f.Field)
		if !ok {
			s.logger.WarnContext(ctx, "QueryLogs: unknown filter field, skipping", slog.String("field", f.Field))
			continue
		}

		clause, err := buildFilterClause(col, f, args)
		if err != nil {
			s.logger.WarnContext(ctx, "QueryLogs: invalid filter, skipping",
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
