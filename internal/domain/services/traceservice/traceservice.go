package traceservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	QueryTraces(ctx context.Context, input QueryTracesInput) (*QueryTracesOutput, apperr.Error)
	GetTrace(ctx context.Context, orgID string, traceID string) (*GetTraceOutput, apperr.Error)
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

type QueryTracesInput struct {
	OrganizationID string
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Cursor         *time.Time
	Filters        []Filter
	SearchQuery    string // Search on span name
	MinDurationMs  int64  // Minimum trace duration in ms
	MaxDurationMs  int64  // Maximum trace duration in ms
}

type QueryTracesOutput struct {
	Traces     []models.TraceSummary `json:"traces"`
	NextCursor *time.Time            `json:"next_cursor,omitempty"`
}

type GetTraceOutput struct {
	Spans []models.Span `json:"spans"`
}
