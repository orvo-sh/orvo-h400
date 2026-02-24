package models

import "time"

// MetricType represents the type of metric point.
type MetricType int8

const (
	MetricTypeSum       MetricType = 1
	MetricTypeGauge     MetricType = 2
	MetricTypeHistogram MetricType = 3
)

func (t MetricType) String() string {
	switch t {
	case MetricTypeSum:
		return "sum"
	case MetricTypeGauge:
		return "gauge"
	case MetricTypeHistogram:
		return "histogram"
	default:
		return "unknown"
	}
}

// AggregationTemporality indicates whether a metric is delta or cumulative.
type AggregationTemporality int8

const (
	AggTemporalityUnspecified AggregationTemporality = 0
	AggTemporalityDelta       AggregationTemporality = 1
	AggTemporalityCumulative  AggregationTemporality = 2
)

func (a AggregationTemporality) String() string {
	switch a {
	case AggTemporalityDelta:
		return "delta"
	case AggTemporalityCumulative:
		return "cumulative"
	default:
		return "unspecified"
	}
}

// MetricExemplar links a metric data point to a trace span.
type MetricExemplar struct {
	TraceID   string    `json:"trace_id"`
	SpanID    string    `json:"span_id"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// MetricPoint represents a single metric data point that can be any of the supported types.
// For Sum/Gauge, ValueInt or ValueDouble is set.
// For Histogram, the Histogram* fields are set.
type MetricPoint struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	MetricName     string            `json:"metric_name"`
	MetricType     MetricType        `json:"metric_type"`
	MetricUnit     string            `json:"metric_unit"`
	Description    string            `json:"description"`
	ServiceName    string            `json:"service_name"`
	DeploymentEnv  string            `json:"deployment_environment"`
	ResourceAttrs  map[string]string `json:"resource_attributes"`
	ScopeName      string            `json:"scope_name"`
	ScopeVersion   string            `json:"scope_version"`
	Attributes     map[string]string `json:"attributes"`

	// Timestamps
	StartTime time.Time `json:"start_time"`
	Time      time.Time `json:"time"`

	// Sum / Gauge value (one of these will be set)
	ValueInt    *int64   `json:"value_int,omitempty"`
	ValueDouble *float64 `json:"value_double,omitempty"`

	// Sum-specific
	AggregationTemporality AggregationTemporality `json:"aggregation_temporality"`
	IsMonotonic            bool                   `json:"is_monotonic"`

	// Histogram fields (also used for converted ExponentialHistograms)
	HistogramCount          *uint64   `json:"histogram_count,omitempty"`
	HistogramSum            *float64  `json:"histogram_sum,omitempty"`
	HistogramMin            *float64  `json:"histogram_min,omitempty"`
	HistogramMax            *float64  `json:"histogram_max,omitempty"`
	HistogramBucketCounts   []uint64  `json:"histogram_bucket_counts,omitempty"`
	HistogramExplicitBounds []float64 `json:"histogram_explicit_bounds,omitempty"`

	// Exemplars
	Exemplars []MetricExemplar `json:"exemplars,omitempty"`

	// Flags
	Flags uint32 `json:"flags"`
}

// TimeseriesPoint represents a single point in a queried timeseries.
type TimeseriesPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// Timeseries represents a labeled set of time-ordered data points.
type Timeseries struct {
	Labels map[string]string `json:"labels"`
	Points []TimeseriesPoint `json:"points"`
}

// MetricMeta represents metadata about a metric (for catalog/discovery).
type MetricMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	ServiceName string `json:"service_name"`
}

// REDMetrics represents Rate, Error, Duration metrics for a service.
type REDMetrics struct {
	RequestRate []TimeseriesPoint `json:"request_rate"`
	ErrorRate   []TimeseriesPoint `json:"error_rate"`
	P50Latency  []TimeseriesPoint `json:"p50_latency"`
	P90Latency  []TimeseriesPoint `json:"p90_latency"`
	P95Latency  []TimeseriesPoint `json:"p95_latency"`
	P99Latency  []TimeseriesPoint `json:"p99_latency"`
}

// DashboardPanel represents a single panel in a custom dashboard.
type DashboardPanel struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Type          string                 `json:"type"` // "timeseries", "stat", "table", "bar", "heatmap"
	Query         DashboardPanelQuery    `json:"query"`
	Visualization map[string]interface{} `json:"visualization,omitempty"`
}

// DashboardPanelQuery defines the metric query for a dashboard panel.
type DashboardPanelQuery struct {
	MetricName  string            `json:"metric_name"`
	Filters     map[string]string `json:"filters,omitempty"`
	GroupBy     []string          `json:"group_by,omitempty"`
	Aggregation string            `json:"aggregation"` // "avg", "sum", "min", "max", "rate", "count", "p50", "p90", "p95", "p99"
	Step        string            `json:"step,omitempty"`
}

// DashboardLayout represents a panel's position on a grid.
type DashboardLayout struct {
	PanelID string `json:"panel_id"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
}

// Dashboard represents a user-created custom dashboard.
type Dashboard struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Panels         []DashboardPanel  `json:"panels"`
	Layout         []DashboardLayout `json:"layout"`
	CreatedBy      string            `json:"created_by"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// MetricConfiguration stores user-configurable settings for derived metrics.
type MetricConfiguration struct {
	ID             string                 `json:"id"`
	OrganizationID string                 `json:"organization_id"`
	MetricName     string                 `json:"metric_name"`
	Config         map[string]interface{} `json:"config"`
	Enabled        bool                   `json:"enabled"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}
