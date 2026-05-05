package remediationservice

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func (s *service) loadLogByID(ctx context.Context, organizationID string, logID string) (*models.LogRecord, error) {
	row := s.pg.Pool().QueryRow(ctx, `
		WITH unioned AS (
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
			WHERE l.organization_id = $1
			  AND l.id = $2

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
			WHERE r.organization_id = $1
			  AND r.id = $2
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
		LIMIT 1
	`, organizationID, logID)

	record := &models.LogRecord{}
	var resourceAttrs []byte
	var scopeAttrs []byte
	var logAttrs []byte

	if err := row.Scan(
		&record.ID,
		&record.Timestamp,
		&record.ObservedTimestamp,
		&record.SeverityNumber,
		&record.SeverityText,
		&record.Body,
		&record.TraceID,
		&record.SpanID,
		&record.TraceFlags,
		&resourceAttrs,
		&record.ResourceSchemaURL,
		&record.ScopeName,
		&record.ScopeVersion,
		&scopeAttrs,
		&record.ScopeSchemaURL,
		&logAttrs,
		&record.ServiceName,
		&record.DeploymentEnvironment,
		&record.OrganizationID,
	); err != nil {
		return nil, err
	}

	if err := parseJSONMap(resourceAttrs, &record.ResourceAttributes); err != nil {
		return nil, err
	}
	if err := parseJSONMap(scopeAttrs, &record.ScopeAttributes); err != nil {
		return nil, err
	}
	if err := parseJSONMap(logAttrs, &record.LogAttributes); err != nil {
		return nil, err
	}

	record.ServiceName = serviceNameFromLog(record)
	return record, nil
}

func (s *service) loadNearbyServiceErrors(
	ctx context.Context,
	organizationID string,
	serviceName string,
	incidentTime time.Time,
	excludeLogID string,
) ([]models.LogRecord, error) {
	if strings.TrimSpace(serviceName) == "" {
		return nil, nil
	}

	start := incidentTime.Add(-s.config.ContextWindow)
	end := incidentTime.Add(s.config.ContextWindow)

	rows, err := s.pg.Pool().Query(ctx, `
		WITH unioned AS (
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
			WHERE l.organization_id = $1
			  AND l.service_name = $2
			  AND l.severity_number >= 17
			  AND l.timestamp >= $3
			  AND l.timestamp <= $4

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
			WHERE r.organization_id = $1
			  AND r.service_name = $2
			  AND r.severity_number >= 17
			  AND r.timestamp >= $3
			  AND r.timestamp <= $4
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
		WHERE id <> $5
		ORDER BY timestamp DESC
		LIMIT $6
	`, organizationID, serviceName, start, end, excludeLogID, s.config.NearbyErrorLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.LogRecord, 0)
	for rows.Next() {
		var record models.LogRecord
		var resourceAttrs []byte
		var scopeAttrs []byte
		var logAttrs []byte
		if err := rows.Scan(
			&record.ID,
			&record.Timestamp,
			&record.ObservedTimestamp,
			&record.SeverityNumber,
			&record.SeverityText,
			&record.Body,
			&record.TraceID,
			&record.SpanID,
			&record.TraceFlags,
			&resourceAttrs,
			&record.ResourceSchemaURL,
			&record.ScopeName,
			&record.ScopeVersion,
			&scopeAttrs,
			&record.ScopeSchemaURL,
			&logAttrs,
			&record.ServiceName,
			&record.DeploymentEnvironment,
			&record.OrganizationID,
		); err != nil {
			return nil, err
		}

		if err := parseJSONMap(resourceAttrs, &record.ResourceAttributes); err != nil {
			return nil, err
		}
		if err := parseJSONMap(scopeAttrs, &record.ScopeAttributes); err != nil {
			return nil, err
		}
		if err := parseJSONMap(logAttrs, &record.LogAttributes); err != nil {
			return nil, err
		}
		record.ServiceName = serviceNameFromLog(&record)
		out = append(out, record)
	}

	return out, rows.Err()
}

func (s *service) loadLatestErrorLogForService(
	ctx context.Context,
	organizationID string,
	serviceName string,
	since time.Time,
) (*models.LogRecord, error) {
	row := s.pg.Pool().QueryRow(ctx, `
		WITH unioned AS (
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
			WHERE l.organization_id = $1
			  AND l.service_name = $2
			  AND l.timestamp >= $3
			  AND (
				l.severity_number >= 17
				OR lower(coalesce(l.severity_text, '')) IN ('error', 'fatal')
			  )

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
			WHERE r.organization_id = $1
			  AND r.service_name = $2
			  AND r.timestamp >= $3
			  AND (
				r.severity_number >= 17
				OR lower(coalesce(r.severity_text, '')) IN ('error', 'fatal')
			  )
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
		LIMIT 1
	`, organizationID, serviceName, since)

	record := &models.LogRecord{}
	var resourceAttrs []byte
	var scopeAttrs []byte
	var logAttrs []byte
	if err := row.Scan(
		&record.ID,
		&record.Timestamp,
		&record.ObservedTimestamp,
		&record.SeverityNumber,
		&record.SeverityText,
		&record.Body,
		&record.TraceID,
		&record.SpanID,
		&record.TraceFlags,
		&resourceAttrs,
		&record.ResourceSchemaURL,
		&record.ScopeName,
		&record.ScopeVersion,
		&scopeAttrs,
		&record.ScopeSchemaURL,
		&logAttrs,
		&record.ServiceName,
		&record.DeploymentEnvironment,
		&record.OrganizationID,
	); err != nil {
		return nil, err
	}

	if err := parseJSONMap(resourceAttrs, &record.ResourceAttributes); err != nil {
		return nil, err
	}
	if err := parseJSONMap(scopeAttrs, &record.ScopeAttributes); err != nil {
		return nil, err
	}
	if err := parseJSONMap(logAttrs, &record.LogAttributes); err != nil {
		return nil, err
	}
	record.ServiceName = serviceNameFromLog(record)

	return record, nil
}

func parseJSONMap(raw []byte, out *map[string]string) error {
	if len(raw) == 0 {
		*out = map[string]string{}
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if parsed == nil {
		*out = map[string]string{}
		return nil
	}

	typed := make(map[string]string, len(parsed))
	for k, v := range parsed {
		switch x := v.(type) {
		case string:
			typed[k] = x
		case nil:
			typed[k] = ""
		default:
			typed[k] = strings.TrimSpace(toJSONString(x))
		}
	}
	*out = typed
	return nil
}

func toJSONString(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func isErrorLog(record *models.LogRecord) bool {
	if record == nil {
		return false
	}
	if record.SeverityNumber >= 17 {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(record.SeverityText)) {
	case "error", "fatal":
		return true
	default:
		return false
	}
}
