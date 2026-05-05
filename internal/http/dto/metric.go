package dto

import (
	"github.com/orvo-sh/orvo/internal/domain/models"
)

// --- Metric Catalog ---

type GetMetricCatalogInput struct {
	OrganizationID string `path:"organization_id"`
	Service        string `query:"service" doc:"Filter by service name" required:"false"`
	Search         string `query:"search" doc:"Search on metric name" required:"false"`
}

type GetMetricCatalogOutput struct {
	Body struct {
		Metrics []models.MetricMeta `json:"metrics"`
	}
}

// --- Metric Timeseries ---

type QueryTimeseriesInput struct {
	OrganizationID string `path:"organization_id"`
	MetricName     string `query:"metric" doc:"Metric name to query" required:"true"`
	Start          string `query:"start" doc:"Start time (RFC3339)" required:"false"`
	End            string `query:"end" doc:"End time (RFC3339)" required:"false"`
	Step           string `query:"step" doc:"Time step (1m, 5m, 15m, 30m, 1h, 6h, 12h, 1d)" required:"false"`
	Aggregation    string `query:"aggregation" doc:"Aggregation function (avg, sum, min, max, rate, count, last, p50, p90, p95, p99)" required:"false"`
	Service        string `query:"service" doc:"Filter by service name" required:"false"`
	GroupBy        string `query:"group_by" doc:"Comma-separated attribute keys to group by" required:"false"`
	Filters        string `query:"filters" doc:"Attribute filters as key=value pairs separated by commas (e.g. span.name=GET /api,span.kind=2)" required:"false"`
}

type QueryTimeseriesOutput struct {
	Body struct {
		Series []models.Timeseries `json:"series"`
	}
}

// --- RED Metrics ---

type GetREDMetricsInput struct {
	OrganizationID string `path:"organization_id"`
	Service        string `query:"service" doc:"Service name" required:"true"`
	Start          string `query:"start" doc:"Start time (RFC3339)" required:"false"`
	End            string `query:"end" doc:"End time (RFC3339)" required:"false"`
	Step           string `query:"step" doc:"Time step (1m, 5m, 15m, 30m, 1h, 6h, 12h, 1d)" required:"false"`
}

type GetREDMetricsOutput struct {
	Body models.REDMetrics
}

// --- Metric Summary (instant value) ---

type GetMetricSummaryInput struct {
	OrganizationID string `path:"organization_id"`
	MetricName     string `query:"metric" doc:"Metric name" required:"true"`
	Aggregation    string `query:"aggregation" doc:"Aggregation function (avg, sum, min, max, last, count, p50, p90, p95, p99)" required:"false"`
	Service        string `query:"service" doc:"Filter by service name" required:"false"`
	Lookback       string `query:"lookback" doc:"Lookback window (e.g. 5m, 1h, 1d)" required:"false"`
	Filters        string `query:"filters" doc:"Attribute filters as key=value pairs separated by commas" required:"false"`
}

type GetMetricSummaryOutput struct {
	Body struct {
		Value     float64 `json:"value"`
		Timestamp string  `json:"timestamp"`
	}
}

// --- Derived Metrics Recalculation ---

type RecalculateDerivedMetricsInput struct {
	OrganizationID string `path:"organization_id"`
	Service        string `query:"service" doc:"Filter by service name" required:"false"`
	Lookback       string `query:"lookback" doc:"Lookback window to recalculate (e.g. 15m, 1h, 24h). Defaults to 1h." required:"false"`
}

type RecalculateDerivedMetricsOutput struct {
	Body struct {
		OrganizationID      string `json:"organization_id"`
		ServiceName         string `json:"service_name,omitempty"`
		WindowStart         string `json:"window_start"`
		WindowEnd           string `json:"window_end"`
		AsOf                string `json:"as_of"`
		RequestSpanCount    int64  `json:"request_span_count"`
		ErrorSpanCount      int64  `json:"error_span_count"`
		ErrorLogCount       int64  `json:"error_log_count"`
		CombinedErrorEvents int64  `json:"combined_error_events"`
	}
}
