package models

type Log struct {
	ID             string         `json:"id"`
	Timestamp      string         `json:"timestamp"`
	Level          string         `json:"level"`
	Message        string         `json:"message"`
	Service        string         `json:"service"`
	Environment    string         `json:"environment"`
	OrganizationID string         `json:"organization_id"`
	TraceID        string         `json:"trace_id"`
	SpanID         string         `json:"span_id"`
	ParentID       string         `json:"parent_id"`
	Attributes     map[string]any `json:"attributes"`
}
