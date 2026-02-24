package traceservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	QueryTraces(ctx context.Context, input QueryTracesInput) (*QueryTracesOutput, apperr.Error)
	GetTrace(ctx context.Context, orgID string, traceID string) (*GetTraceOutput, apperr.Error)
	GetServices(ctx context.Context, organizationID string) ([]string, apperr.Error)
	GetServiceMap(ctx context.Context, organizationID string) ([]models.ServiceEdge, apperr.Error)
	GetSources(ctx context.Context, organizationID string) ([]models.ServiceSource, apperr.Error)
}

type service struct {
	pg           *postgres.DB
	logger       *slog.Logger
	hotRetention time.Duration
}

type Config struct {
	HotRetention time.Duration
}

func New(pg *postgres.DB, logger *slog.Logger, cfg ...Config) Service {
	config := Config{
		HotRetention: 7 * 24 * time.Hour,
	}
	if len(cfg) > 0 && cfg[0].HotRetention > 0 {
		config.HotRetention = cfg[0].HotRetention
	}

	return &service{
		pg:           pg,
		logger:       logger,
		hotRetention: config.HotRetention,
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
