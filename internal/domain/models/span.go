package models

import "time"

// SpanKind represents the type of span (matches OTLP SpanKind enum).
type SpanKind uint8

const (
	SpanKindUnspecified SpanKind = 0
	SpanKindInternal    SpanKind = 1
	SpanKindServer      SpanKind = 2
	SpanKindClient      SpanKind = 3
	SpanKindProducer    SpanKind = 4
	SpanKindConsumer    SpanKind = 5
)

func (k SpanKind) String() string {
	switch k {
	case SpanKindInternal:
		return "Internal"
	case SpanKindServer:
		return "Server"
	case SpanKindClient:
		return "Client"
	case SpanKindProducer:
		return "Producer"
	case SpanKindConsumer:
		return "Consumer"
	default:
		return "Unspecified"
	}
}

// SpanStatusCode represents the status of a span (matches OTLP StatusCode enum).
type SpanStatusCode uint8

const (
	SpanStatusUnset SpanStatusCode = 0
	SpanStatusOk    SpanStatusCode = 1
	SpanStatusError SpanStatusCode = 2
)

func (s SpanStatusCode) String() string {
	switch s {
	case SpanStatusOk:
		return "Ok"
	case SpanStatusError:
		return "Error"
	default:
		return "Unset"
	}
}

// SpanEvent represents an event that occurred during a span's lifetime.
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes"`
}

// SpanLink represents a link to another span.
type SpanLink struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	TraceState string            `json:"trace_state"`
	Attributes map[string]string `json:"attributes"`
}

// Span represents a single span in a distributed trace (OTLP trace data model).
type Span struct {
	ID                    string            `json:"id"`
	OrganizationID        string            `json:"organization_id"`
	TraceID               string            `json:"trace_id"`
	SpanID                string            `json:"span_id"`
	ParentSpanID          string            `json:"parent_span_id"`
	TraceState            string            `json:"trace_state"`
	Name                  string            `json:"name"`
	Kind                  uint8             `json:"kind"`
	StartTime             time.Time         `json:"start_time"`
	EndTime               time.Time         `json:"end_time"`
	DurationNs            int64             `json:"duration_ns"`
	StatusCode            uint8             `json:"status_code"`
	StatusMessage         string            `json:"status_message"`
	ResourceAttributes    map[string]string `json:"resource_attributes"`
	ScopeAttributes       map[string]string `json:"scope_attributes"`
	SpanAttributes        map[string]string `json:"span_attributes"`
	ResourceSchemaURL     string            `json:"resource_schema_url"`
	ScopeName             string            `json:"scope_name"`
	ScopeVersion          string            `json:"scope_version"`
	ScopeSchemaURL        string            `json:"scope_schema_url"`
	Events                []SpanEvent       `json:"events"`
	Links                 []SpanLink        `json:"links"`
	ServiceName           string            `json:"service_name"`
	DeploymentEnvironment string            `json:"deployment_environment"`
}

// TraceSummary represents an aggregated view of a trace for listing purposes.
type TraceSummary struct {
	TraceID      string    `json:"trace_id"`
	RootSpanName string    `json:"root_span_name"`
	RootService  string    `json:"root_service"`
	StartTime    time.Time `json:"start_time"`
	DurationNs   int64     `json:"duration_ns"`
	SpanCount    uint64    `json:"span_count"`
	ErrorCount   uint64    `json:"error_count"`
}
