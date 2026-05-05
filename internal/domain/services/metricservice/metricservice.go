package metricservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	QueryTimeseries(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error)
	GetMetricCatalog(ctx context.Context, input GetMetricCatalogInput) (*GetMetricCatalogOutput, apperr.Error)
	GetREDMetrics(ctx context.Context, input GetREDMetricsInput) (*models.REDMetrics, apperr.Error)
	GetMetricSummary(ctx context.Context, input GetMetricSummaryInput) (*GetMetricSummaryOutput, apperr.Error)
	RecalculateDerivedMetrics(ctx context.Context, input RecalculateDerivedMetricsInput) (*RecalculateDerivedMetricsOutput, apperr.Error)
}

type service struct {
	pg     *postgres.DB
	logger *slog.Logger
}

func New(pg *postgres.DB, logger *slog.Logger) Service {
	return &service{
		pg:     pg,
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

// RecalculateDerivedMetricsInput defines parameters for forcing a derived metrics recomputation window.
type RecalculateDerivedMetricsInput struct {
	OrganizationID string
	ServiceName    string
	LookbackWindow time.Duration
}

type RecalculateDerivedMetricsOutput struct {
	OrganizationID      string    `json:"organization_id"`
	ServiceName         string    `json:"service_name,omitempty"`
	WindowStart         time.Time `json:"window_start"`
	WindowEnd           time.Time `json:"window_end"`
	AsOf                time.Time `json:"as_of"`
	RequestSpanCount    int64     `json:"request_span_count"`
	ErrorSpanCount      int64     `json:"error_span_count"`
	ErrorLogCount       int64     `json:"error_log_count"`
	CombinedErrorEvents int64     `json:"combined_error_events"`
}
