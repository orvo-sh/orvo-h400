package dto

import (
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

// QueryLogsInput represents the HTTP request for querying logs.
type QueryLogsInput struct {
	OrganizationID string `path:"organization_id"`
	Start          string `query:"start" doc:"Start time (RFC3339)" required:"false"`
	End            string `query:"end" doc:"End time (RFC3339)" required:"false"`
	Limit          int    `query:"limit" doc:"Max results to return (default 100, max 1000)" required:"false"`
	Cursor         string `query:"cursor" doc:"Pagination cursor (RFC3339Nano timestamp)" required:"false"`
	Search         string `query:"search" doc:"Full-text search on log body" required:"false"`
	Service        string `query:"service" doc:"Filter by service name (comma-separated)" required:"false"`
	Severity       string `query:"severity" doc:"Filter by severity (comma-separated: debug,info,warn,error,fatal)" required:"false"`
}

type QueryLogsOutput struct {
	Body struct {
		Logs       []models.LogRecord `json:"logs"`
		NextCursor *time.Time         `json:"next_cursor,omitempty"`
	}
}

// GetHistogramInput represents the HTTP request for the log histogram.
type GetHistogramInput struct {
	OrganizationID string `path:"organization_id"`
	Start          string `query:"start" doc:"Start time (RFC3339)" required:"false"`
	End            string `query:"end" doc:"End time (RFC3339)" required:"false"`
	Interval       string `query:"interval" doc:"Bucket interval (1m, 5m, 15m, 30m, 1h, 6h, 12h, 1d)" required:"false"`
	Search         string `query:"search" doc:"Full-text search on log body" required:"false"`
	Service        string `query:"service" doc:"Filter by service name (comma-separated)" required:"false"`
	Severity       string `query:"severity" doc:"Filter by severity (comma-separated)" required:"false"`
}

type HistogramBucket struct {
	Time  time.Time `json:"time"`
	Count uint64    `json:"count"`
	Debug uint64    `json:"debug"`
	Info  uint64    `json:"info"`
	Warn  uint64    `json:"warn"`
	Error uint64    `json:"error"`
	Fatal uint64    `json:"fatal"`
}

type GetHistogramOutput struct {
	Body struct {
		Buckets []HistogramBucket `json:"buckets"`
	}
}

// GetServicesInput represents the HTTP request for listing available services.
type GetServicesInput struct {
	OrganizationID string `path:"organization_id"`
}

type GetServicesOutput struct {
	Body struct {
		Services []string `json:"services"`
	}
}
