-- +goose Up
DROP TABLE IF EXISTS logs;

CREATE TABLE logs (
    -- Timestamps (nanosecond precision per OTEL spec)
    timestamp DateTime64(9),
    observed_timestamp DateTime64(9),

    -- Severity (OTEL SeverityNumber enum: TRACE=1-4, DEBUG=5-8, INFO=9-12, WARN=13-16, ERROR=17-20, FATAL=21-24)
    severity_number UInt8,
    severity_text LowCardinality(String),

    -- Body (the actual log message)
    body String,

    -- Trace context
    trace_id String DEFAULT '',
    span_id String DEFAULT '',
    trace_flags UInt32 DEFAULT 0,

    -- Resource (who produced this log)
    resource_attributes Map(String, String),
    resource_schema_url String DEFAULT '',

    -- Instrumentation Scope
    scope_name String DEFAULT '',
    scope_version String DEFAULT '',
    scope_attributes Map(String, String),
    scope_schema_url String DEFAULT '',

    -- Log Record attributes
    log_attributes Map(String, String),

    -- Denormalized for query performance (extracted from resource_attributes at ingest time)
    service_name LowCardinality(String),
    deployment_environment LowCardinality(String),

    -- Multi-tenant
    organization_id String
) ENGINE = MergeTree
PARTITION BY toDate(timestamp)
ORDER BY (organization_id, service_name, severity_number, timestamp)
SETTINGS index_granularity = 8192;

-- +goose Down
DROP TABLE IF EXISTS logs;
