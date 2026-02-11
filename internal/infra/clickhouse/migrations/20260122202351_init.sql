-- +goose Up
CREATE TABLE logs (
    id String,
    organization_id String,
    timestamp DateTime64 (9),
    observed_timestamp DateTime64 (9),
    severity_number UInt8,
    severity_text LowCardinality (String),
    body String,
    trace_id String DEFAULT '',
    span_id String DEFAULT '',
    trace_flags UInt32 DEFAULT 0,
    resource_attributes Map (String, String),
    scope_attributes Map (String, String),
    log_attributes Map (String, String),
    resource_schema_url String DEFAULT '',
    scope_name String DEFAULT '',
    scope_version String DEFAULT '',
    scope_schema_url String DEFAULT '',
    service_name LowCardinality (String),
    deployment_environment LowCardinality (String)
) ENGINE = MergeTree
PARTITION BY
    toDate (timestamp)
ORDER BY (
        organization_id, service_name, severity_number, timestamp
    ) SETTINGS index_granularity = 8192;

-- +goose Down
DROP TABLE IF EXISTS logs;