package models

import "time"

type LogRecord struct {
	ID                    string            `json:"id"`
	Timestamp             time.Time         `json:"timestamp"`
	ObservedTimestamp     time.Time         `json:"observed_timestamp"`
	SeverityNumber        uint8             `json:"severity_number"`
	SeverityText          string            `json:"severity_text"`
	Body                  string            `json:"body"`
	TraceID               string            `json:"trace_id"`
	SpanID                string            `json:"span_id"`
	TraceFlags            uint32            `json:"trace_flags"`
	ResourceAttributes    map[string]string `json:"resource_attributes"`
	ResourceSchemaURL     string            `json:"resource_schema_url"`
	ScopeName             string            `json:"scope_name"`
	ScopeVersion          string            `json:"scope_version"`
	ScopeAttributes       map[string]string `json:"scope_attributes"`
	ScopeSchemaURL        string            `json:"scope_schema_url"`
	LogAttributes         map[string]string `json:"log_attributes"`
	ServiceName           string            `json:"service_name"`
	DeploymentEnvironment string            `json:"deployment_environment"`
	OrganizationID        string            `json:"organization_id"`
}
