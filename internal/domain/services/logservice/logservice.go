package logservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	QueryLogs(ctx context.Context, input QueryLogsInput) (*QueryLogsOutput, apperr.Error)
	GetHistogram(ctx context.Context, input GetHistogramInput) (*GetHistogramOutput, apperr.Error)
	GetServices(ctx context.Context, organizationID string) ([]string, apperr.Error)
}

type service struct {
	ch     *clickhouse.DB
	logger *slog.Logger
}

func New(ch *clickhouse.DB, logger *slog.Logger) Service {
	return &service{
		ch:     ch,
		logger: logger,
	}
}

type FilterOperator string

const (
	FilterOperatorIn          FilterOperator = "IN"
	FilterOperatorNotIn       FilterOperator = "NIN"
	FilterOperatorContains    FilterOperator = "CONTAINS"
	FilterOperatorNotContains FilterOperator = "NCONTAINS"
	FilterOperatorGt          FilterOperator = "GT"
	FilterOperatorGte         FilterOperator = "GTE"
	FilterOperatorLt          FilterOperator = "LT"
	FilterOperatorLte         FilterOperator = "LTE"
)

type Filter struct {
	Field    string
	Operator FilterOperator
	Value    string
}

type QueryLogsInput struct {
	OrganizationID string
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Cursor         *time.Time // Cursor-based pagination: timestamp of last seen record
	Filters        []Filter
	SearchQuery    string // Full-text search on body
}

type QueryLogsOutput struct {
	Logs       []models.LogRecord `json:"logs"`
	NextCursor *time.Time         `json:"next_cursor,omitempty"`
}

type GetHistogramInput struct {
	OrganizationID string
	StartTime      time.Time
	EndTime        time.Time
	Interval       string // e.g. "5m", "1h"
	Filters        []Filter
	SearchQuery    string
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
	Buckets []HistogramBucket
}
