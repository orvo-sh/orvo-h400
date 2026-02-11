package metricservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	QueryTimeseries(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error)
	GetMetricCatalog(ctx context.Context, input GetMetricCatalogInput) (*GetMetricCatalogOutput, apperr.Error)
	GetREDMetrics(ctx context.Context, input GetREDMetricsInput) (*models.REDMetrics, apperr.Error)
	GetMetricSummary(ctx context.Context, input GetMetricSummaryInput) (*GetMetricSummaryOutput, apperr.Error)
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

// QueryTimeseriesInput defines parameters for querying metric timeseries.
type QueryTimeseriesInput struct {
	OrganizationID string
	MetricName     string
	StartTime      time.Time
	EndTime        time.Time
	Step           string            // e.g. "1m", "5m", "1h", "1d" — auto-selected if empty
	Aggregation    string            // "avg", "sum", "min", "max", "rate", "count", "p50", "p90", "p95", "p99", "last"
	Filters        map[string]string // attribute filters (key=value)
	GroupBy        []string          // attribute keys to group by
	ServiceName    string            // optional convenience filter
}

type QueryTimeseriesOutput struct {
	Series []models.Timeseries `json:"series"`
}

// GetMetricCatalogInput defines parameters for listing available metrics.
type GetMetricCatalogInput struct {
	OrganizationID string
	ServiceName    string // optional filter
	SearchQuery    string // optional search on metric name
}

type GetMetricCatalogOutput struct {
	Metrics []models.MetricMeta `json:"metrics"`
}

// GetREDMetricsInput defines parameters for fetching RED metrics for a service.
type GetREDMetricsInput struct {
	OrganizationID string
	ServiceName    string
	StartTime      time.Time
	EndTime        time.Time
	Step           string // e.g. "1m", "5m", "1h" — auto-selected if empty
}

// GetMetricSummaryInput defines parameters for an instant metric value.
type GetMetricSummaryInput struct {
	OrganizationID string
	MetricName     string
	Aggregation    string            // "avg", "sum", "min", "max", "last", "count"
	Filters        map[string]string // attribute filters
	ServiceName    string
	LookbackWindow time.Duration // how far back to look (default 5m)
}

type GetMetricSummaryOutput struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}
