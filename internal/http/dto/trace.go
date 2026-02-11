package dto

import (
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

// QueryTracesInput represents the HTTP request for querying traces.
type QueryTracesInput struct {
	OrganizationID string `path:"organization_id"`
	Start          string `query:"start" doc:"Start time (RFC3339)" required:"false"`
	End            string `query:"end" doc:"End time (RFC3339)" required:"false"`
	Limit          int    `query:"limit" doc:"Max results to return (default 50, max 500)" required:"false"`
	Cursor         string `query:"cursor" doc:"Pagination cursor (RFC3339Nano timestamp)" required:"false"`
	Search         string `query:"search" doc:"Search on span name" required:"false"`
	Service        string `query:"service" doc:"Filter by service name (comma-separated)" required:"false"`
	Status         string `query:"status" doc:"Filter by status code (comma-separated: ok,error,unset)" required:"false"`
	MinDuration    int64  `query:"min_duration" doc:"Minimum trace duration in milliseconds" required:"false"`
	MaxDuration    int64  `query:"max_duration" doc:"Maximum trace duration in milliseconds" required:"false"`
}

type QueryTracesOutput struct {
	Body struct {
		Traces     []models.TraceSummary `json:"traces"`
		NextCursor *time.Time            `json:"next_cursor,omitempty"`
	}
}

// GetTraceInput represents the HTTP request for getting a single trace.
type GetTraceInput struct {
	OrganizationID string `path:"organization_id"`
	TraceID        string `path:"trace_id"`
}

type GetTraceOutput struct {
	Body struct {
		Spans []models.Span `json:"spans"`
	}
}

// GetTraceServicesInput represents the HTTP request for listing available services from traces.
type GetTraceServicesInput struct {
	OrganizationID string `path:"organization_id"`
}

type GetTraceServicesOutput struct {
	Body struct {
		Services []string `json:"services"`
	}
}

// GetServiceMapInput represents the HTTP request for the service dependency map.
type GetServiceMapInput struct {
	OrganizationID string `path:"organization_id"`
}

type GetServiceMapOutput struct {
	Body struct {
		Edges []models.ServiceEdge `json:"edges"`
	}
}

// GetSourcesInput represents the HTTP request for listing trace sources.
type GetSourcesInput struct {
	OrganizationID string `path:"organization_id"`
}

type GetSourcesOutput struct {
	Body struct {
		Sources []models.ServiceSource `json:"sources"`
	}
}
