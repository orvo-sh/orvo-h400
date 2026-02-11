-- +goose Up
-- +goose StatementBegin

-- Raw metrics table: single wide table for all OTLP metric types (Sum, Gauge, Histogram).
-- ExponentialHistograms are converted to regular Histograms on ingestion.
CREATE TABLE metrics (
    organization_id String,

    -- Metric identity
    metric_name LowCardinality (String),
    metric_type Enum8 ('sum' = 1, 'gauge' = 2, 'histogram' = 3),
    metric_unit LowCardinality (String) DEFAULT '',
    metric_description String DEFAULT '',

    -- Dimensions
    service_name LowCardinality (String),
    deployment_environment LowCardinality (String) DEFAULT '',
    resource_attributes Map (LowCardinality(String), String),
    scope_name LowCardinality (String) DEFAULT '',
    scope_version String DEFAULT '',
    attributes Map (LowCardinality(String), String),

    -- Timestamps
    start_time DateTime64 (9) DEFAULT toDateTime64(0, 9),
    time DateTime64 (9),

    -- Sum / Gauge value (one of these will be set)
    value_int Nullable (Int64),
    value_double Nullable (Float64),

    -- Sum-specific fields
    aggregation_temporality Enum8 ('unspecified' = 0, 'delta' = 1, 'cumulative' = 2) DEFAULT 'unspecified',
    is_monotonic Bool DEFAULT false,

    -- Histogram fields (also used for converted ExponentialHistograms)
    histogram_count Nullable (UInt64),
    histogram_sum Nullable (Float64),
    histogram_min Nullable (Float64),
    histogram_max Nullable (Float64),
    histogram_bucket_counts Array (UInt64),
    histogram_explicit_bounds Array (Float64),

    -- Exemplars (linking metrics to traces)
    exemplar_trace_ids Array (String),
    exemplar_span_ids Array (String),
    exemplar_values Array (Float64),
    exemplar_timestamps Array (DateTime64 (9)),

    -- Flags
    flags UInt32 DEFAULT 0
) ENGINE = MergeTree
PARTITION BY
    toDate (time)
ORDER BY
    (
        organization_id,
        metric_name,
        service_name,
        attributes,
        time
    )
TTL toDate(time) + INTERVAL 30 DAY
SETTINGS
    index_granularity = 8192;

-- 1-minute rollup target table
CREATE TABLE metrics_1m (
    organization_id String,
    metric_name LowCardinality (String),
    metric_type Enum8 ('sum' = 1, 'gauge' = 2, 'histogram' = 3),
    service_name LowCardinality (String),
    attributes Map (LowCardinality(String), String),
    time_bucket DateTime,

    -- Aggregated scalar values (for Sum / Gauge)
    last_value AggregateFunction (argMax, Float64, DateTime64 (9)),
    min_value SimpleAggregateFunction (min, Float64),
    max_value SimpleAggregateFunction (max, Float64),
    sum_value SimpleAggregateFunction (sum, Float64),
    avg_value AggregateFunction (avg, Float64),
    point_count SimpleAggregateFunction (sum, UInt64),

    -- Aggregated histogram values
    histogram_bucket_counts SimpleAggregateFunction (sumForEach, Array (UInt64)),
    histogram_count SimpleAggregateFunction (sum, UInt64),
    histogram_sum SimpleAggregateFunction (sum, Float64),
    histogram_min SimpleAggregateFunction (min, Float64),
    histogram_max SimpleAggregateFunction (max, Float64),
    histogram_explicit_bounds Array (Float64)
) ENGINE = AggregatingMergeTree
PARTITION BY
    toDate (time_bucket)
ORDER BY
    (
        organization_id,
        metric_name,
        service_name,
        attributes,
        time_bucket
    )
TTL toDate(time_bucket) + INTERVAL 90 DAY
SETTINGS
    index_granularity = 8192;

-- Materialized view: raw metrics -> 1-minute rollup
CREATE MATERIALIZED VIEW metrics_1m_mv TO metrics_1m AS
SELECT
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    toStartOfMinute (time) AS time_bucket,
    argMaxState (coalesce (value_double, CAST(value_int AS Float64), 0), time) AS last_value,
    min (coalesce (value_double, CAST(value_int AS Float64), 0)) AS min_value,
    max (coalesce (value_double, CAST(value_int AS Float64), 0)) AS max_value,
    sum (coalesce (value_double, CAST(value_int AS Float64), 0)) AS sum_value,
    avgState (coalesce (value_double, CAST(value_int AS Float64), 0)) AS avg_value,
    count () AS point_count,
    sumForEach (histogram_bucket_counts) AS histogram_bucket_counts,
    sum (histogram_count) AS histogram_count,
    sum (histogram_sum) AS histogram_sum,
    min (histogram_min) AS histogram_min,
    max (histogram_max) AS histogram_max,
    any (histogram_explicit_bounds) AS histogram_explicit_bounds
FROM
    metrics
GROUP BY
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    time_bucket;

-- 1-hour rollup target table
CREATE TABLE metrics_1h (
    organization_id String,
    metric_name LowCardinality (String),
    metric_type Enum8 ('sum' = 1, 'gauge' = 2, 'histogram' = 3),
    service_name LowCardinality (String),
    attributes Map (LowCardinality(String), String),
    time_bucket DateTime,

    last_value AggregateFunction (argMax, Float64, DateTime64 (9)),
    min_value SimpleAggregateFunction (min, Float64),
    max_value SimpleAggregateFunction (max, Float64),
    sum_value SimpleAggregateFunction (sum, Float64),
    avg_value AggregateFunction (avg, Float64),
    point_count SimpleAggregateFunction (sum, UInt64),

    histogram_bucket_counts SimpleAggregateFunction (sumForEach, Array (UInt64)),
    histogram_count SimpleAggregateFunction (sum, UInt64),
    histogram_sum SimpleAggregateFunction (sum, Float64),
    histogram_min SimpleAggregateFunction (min, Float64),
    histogram_max SimpleAggregateFunction (max, Float64),
    histogram_explicit_bounds Array (Float64)
) ENGINE = AggregatingMergeTree
PARTITION BY
    toYYYYMM (time_bucket)
ORDER BY
    (
        organization_id,
        metric_name,
        service_name,
        attributes,
        time_bucket
    )
TTL toDate(time_bucket) + INTERVAL 365 DAY
SETTINGS
    index_granularity = 8192;

-- Materialized view: raw metrics -> 1-hour rollup
CREATE MATERIALIZED VIEW metrics_1h_mv TO metrics_1h AS
SELECT
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    toStartOfHour (time) AS time_bucket,
    argMaxState (coalesce (value_double, CAST(value_int AS Float64), 0), time) AS last_value,
    min (coalesce (value_double, CAST(value_int AS Float64), 0)) AS min_value,
    max (coalesce (value_double, CAST(value_int AS Float64), 0)) AS max_value,
    sum (coalesce (value_double, CAST(value_int AS Float64), 0)) AS sum_value,
    avgState (coalesce (value_double, CAST(value_int AS Float64), 0)) AS avg_value,
    count () AS point_count,
    sumForEach (histogram_bucket_counts) AS histogram_bucket_counts,
    sum (histogram_count) AS histogram_count,
    sum (histogram_sum) AS histogram_sum,
    min (histogram_min) AS histogram_min,
    max (histogram_max) AS histogram_max,
    any (histogram_explicit_bounds) AS histogram_explicit_bounds
FROM
    metrics
GROUP BY
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    time_bucket;

-- 1-day rollup target table
CREATE TABLE metrics_1d (
    organization_id String,
    metric_name LowCardinality (String),
    metric_type Enum8 ('sum' = 1, 'gauge' = 2, 'histogram' = 3),
    service_name LowCardinality (String),
    attributes Map (LowCardinality(String), String),
    time_bucket DateTime,

    last_value AggregateFunction (argMax, Float64, DateTime64 (9)),
    min_value SimpleAggregateFunction (min, Float64),
    max_value SimpleAggregateFunction (max, Float64),
    sum_value SimpleAggregateFunction (sum, Float64),
    avg_value AggregateFunction (avg, Float64),
    point_count SimpleAggregateFunction (sum, UInt64),

    histogram_bucket_counts SimpleAggregateFunction (sumForEach, Array (UInt64)),
    histogram_count SimpleAggregateFunction (sum, UInt64),
    histogram_sum SimpleAggregateFunction (sum, Float64),
    histogram_min SimpleAggregateFunction (min, Float64),
    histogram_max SimpleAggregateFunction (max, Float64),
    histogram_explicit_bounds Array (Float64)
) ENGINE = AggregatingMergeTree
PARTITION BY
    toYYYYMM (time_bucket)
ORDER BY
    (
        organization_id,
        metric_name,
        service_name,
        attributes,
        time_bucket
    )
SETTINGS
    index_granularity = 8192;

-- Materialized view: raw metrics -> 1-day rollup
CREATE MATERIALIZED VIEW metrics_1d_mv TO metrics_1d AS
SELECT
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    toStartOfDay (time) AS time_bucket,
    argMaxState (coalesce (value_double, CAST(value_int AS Float64), 0), time) AS last_value,
    min (coalesce (value_double, CAST(value_int AS Float64), 0)) AS min_value,
    max (coalesce (value_double, CAST(value_int AS Float64), 0)) AS max_value,
    sum (coalesce (value_double, CAST(value_int AS Float64), 0)) AS sum_value,
    avgState (coalesce (value_double, CAST(value_int AS Float64), 0)) AS avg_value,
    count () AS point_count,
    sumForEach (histogram_bucket_counts) AS histogram_bucket_counts,
    sum (histogram_count) AS histogram_count,
    sum (histogram_sum) AS histogram_sum,
    min (histogram_min) AS histogram_min,
    max (histogram_max) AS histogram_max,
    any (histogram_explicit_bounds) AS histogram_explicit_bounds
FROM
    metrics
GROUP BY
    organization_id,
    metric_name,
    metric_type,
    service_name,
    attributes,
    time_bucket;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS metrics_1d_mv;
DROP TABLE IF EXISTS metrics_1d;
DROP VIEW IF EXISTS metrics_1h_mv;
DROP TABLE IF EXISTS metrics_1h;
DROP VIEW IF EXISTS metrics_1m_mv;
DROP TABLE IF EXISTS metrics_1m;
DROP TABLE IF EXISTS metrics;
-- +goose StatementEnd
