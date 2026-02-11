-- +goose Up
-- +goose StatementBegin

-- Derived RED metrics from spans.
-- These MVs insert into the metrics table, which then feeds into the rollup
-- tables (metrics_1m, metrics_1h, metrics_1d) automatically.

-- 1. Request count: one metric point per span.
-- Metric name: spans.request.count
-- Type: sum (delta), monotonic
-- Value: 1 per span
CREATE MATERIALIZED VIEW spans_request_count_mv TO metrics AS
SELECT
    organization_id,
    'spans.request.count' AS metric_name,
    'sum' AS metric_type,
    '' AS metric_unit,
    'Number of span requests' AS metric_description,
    service_name,
    deployment_environment,
    resource_attributes,
    scope_name,
    scope_version,
    map('span.name', name, 'span.kind', toString(kind)) AS attributes,
    start_time,
    start_time AS time,
    CAST(1 AS Nullable(Int64)) AS value_int,
    CAST(NULL AS Nullable(Float64)) AS value_double,
    'delta' AS aggregation_temporality,
    true AS is_monotonic,
    CAST(NULL AS Nullable(UInt64)) AS histogram_count,
    CAST(NULL AS Nullable(Float64)) AS histogram_sum,
    CAST(NULL AS Nullable(Float64)) AS histogram_min,
    CAST(NULL AS Nullable(Float64)) AS histogram_max,
    [] :: Array(UInt64) AS histogram_bucket_counts,
    [] :: Array(Float64) AS histogram_explicit_bounds,
    [] :: Array(String) AS exemplar_trace_ids,
    [] :: Array(String) AS exemplar_span_ids,
    [] :: Array(Float64) AS exemplar_values,
    [] :: Array(DateTime64(9)) AS exemplar_timestamps,
    toUInt32(0) AS flags
FROM spans;

-- 2. Error count: one metric point per span with status_code = 2 (Error).
-- Metric name: spans.error.count
-- Type: sum (delta), monotonic
-- Value: 1 per error span
CREATE MATERIALIZED VIEW spans_error_count_mv TO metrics AS
SELECT
    organization_id,
    'spans.error.count' AS metric_name,
    'sum' AS metric_type,
    '' AS metric_unit,
    'Number of span errors' AS metric_description,
    service_name,
    deployment_environment,
    resource_attributes,
    scope_name,
    scope_version,
    map('span.name', name, 'span.kind', toString(kind)) AS attributes,
    start_time,
    start_time AS time,
    CAST(1 AS Nullable(Int64)) AS value_int,
    CAST(NULL AS Nullable(Float64)) AS value_double,
    'delta' AS aggregation_temporality,
    true AS is_monotonic,
    CAST(NULL AS Nullable(UInt64)) AS histogram_count,
    CAST(NULL AS Nullable(Float64)) AS histogram_sum,
    CAST(NULL AS Nullable(Float64)) AS histogram_min,
    CAST(NULL AS Nullable(Float64)) AS histogram_max,
    [] :: Array(UInt64) AS histogram_bucket_counts,
    [] :: Array(Float64) AS histogram_explicit_bounds,
    [] :: Array(String) AS exemplar_trace_ids,
    [] :: Array(String) AS exemplar_span_ids,
    [] :: Array(Float64) AS exemplar_values,
    [] :: Array(DateTime64(9)) AS exemplar_timestamps,
    toUInt32(0) AS flags
FROM spans
WHERE status_code = 2;

-- 3. Duration: one metric point per span with duration in milliseconds.
-- Metric name: spans.duration
-- Type: gauge
-- Value: duration in milliseconds (float64)
-- The raw points allow quantile computation; rollups provide min/max/avg.
CREATE MATERIALIZED VIEW spans_duration_mv TO metrics AS
SELECT
    organization_id,
    'spans.duration' AS metric_name,
    'gauge' AS metric_type,
    'ms' AS metric_unit,
    'Span duration in milliseconds' AS metric_description,
    service_name,
    deployment_environment,
    resource_attributes,
    scope_name,
    scope_version,
    map('span.name', name, 'span.kind', toString(kind)) AS attributes,
    start_time,
    start_time AS time,
    CAST(NULL AS Nullable(Int64)) AS value_int,
    CAST(duration_ns / 1000000.0 AS Nullable(Float64)) AS value_double,
    'unspecified' AS aggregation_temporality,
    false AS is_monotonic,
    CAST(NULL AS Nullable(UInt64)) AS histogram_count,
    CAST(NULL AS Nullable(Float64)) AS histogram_sum,
    CAST(NULL AS Nullable(Float64)) AS histogram_min,
    CAST(NULL AS Nullable(Float64)) AS histogram_max,
    [] :: Array(UInt64) AS histogram_bucket_counts,
    [] :: Array(Float64) AS histogram_explicit_bounds,
    [trace_id] AS exemplar_trace_ids,
    [span_id] AS exemplar_span_ids,
    [CAST(duration_ns / 1000000.0 AS Float64)] AS exemplar_values,
    [start_time] AS exemplar_timestamps,
    toUInt32(0) AS flags
FROM spans;

-- Derived metrics from logs.

-- 4. Log record count: one metric point per log record.
-- Metric name: logs.record.count
-- Type: sum (delta), monotonic
-- Value: 1 per log record, keyed by severity
CREATE MATERIALIZED VIEW logs_record_count_mv TO metrics AS
SELECT
    organization_id,
    'logs.record.count' AS metric_name,
    'sum' AS metric_type,
    '' AS metric_unit,
    'Number of log records' AS metric_description,
    service_name,
    deployment_environment,
    resource_attributes,
    scope_name,
    scope_version,
    map('severity', severity_text) AS attributes,
    timestamp AS start_time,
    timestamp AS time,
    CAST(1 AS Nullable(Int64)) AS value_int,
    CAST(NULL AS Nullable(Float64)) AS value_double,
    'delta' AS aggregation_temporality,
    true AS is_monotonic,
    CAST(NULL AS Nullable(UInt64)) AS histogram_count,
    CAST(NULL AS Nullable(Float64)) AS histogram_sum,
    CAST(NULL AS Nullable(Float64)) AS histogram_min,
    CAST(NULL AS Nullable(Float64)) AS histogram_max,
    [] :: Array(UInt64) AS histogram_bucket_counts,
    [] :: Array(Float64) AS histogram_explicit_bounds,
    [] :: Array(String) AS exemplar_trace_ids,
    [] :: Array(String) AS exemplar_span_ids,
    [] :: Array(Float64) AS exemplar_values,
    [] :: Array(DateTime64(9)) AS exemplar_timestamps,
    toUInt32(0) AS flags
FROM logs;

-- 5. Log error count: one metric point per error/fatal log record.
-- Metric name: logs.error.count
-- Type: sum (delta), monotonic
-- severity_number >= 17 corresponds to ERROR and above (ERROR=17-20, FATAL=21-24)
CREATE MATERIALIZED VIEW logs_error_count_mv TO metrics AS
SELECT
    organization_id,
    'logs.error.count' AS metric_name,
    'sum' AS metric_type,
    '' AS metric_unit,
    'Number of error log records' AS metric_description,
    service_name,
    deployment_environment,
    resource_attributes,
    scope_name,
    scope_version,
    map('severity', severity_text) AS attributes,
    timestamp AS start_time,
    timestamp AS time,
    CAST(1 AS Nullable(Int64)) AS value_int,
    CAST(NULL AS Nullable(Float64)) AS value_double,
    'delta' AS aggregation_temporality,
    true AS is_monotonic,
    CAST(NULL AS Nullable(UInt64)) AS histogram_count,
    CAST(NULL AS Nullable(Float64)) AS histogram_sum,
    CAST(NULL AS Nullable(Float64)) AS histogram_min,
    CAST(NULL AS Nullable(Float64)) AS histogram_max,
    [] :: Array(UInt64) AS histogram_bucket_counts,
    [] :: Array(Float64) AS histogram_explicit_bounds,
    [] :: Array(String) AS exemplar_trace_ids,
    [] :: Array(String) AS exemplar_span_ids,
    [] :: Array(Float64) AS exemplar_values,
    [] :: Array(DateTime64(9)) AS exemplar_timestamps,
    toUInt32(0) AS flags
FROM logs
WHERE severity_number >= 17;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS logs_error_count_mv;
DROP VIEW IF EXISTS logs_record_count_mv;
DROP VIEW IF EXISTS spans_duration_mv;
DROP VIEW IF EXISTS spans_error_count_mv;
DROP VIEW IF EXISTS spans_request_count_mv;
-- +goose StatementEnd
