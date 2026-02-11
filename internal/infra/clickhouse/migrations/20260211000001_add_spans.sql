-- +goose Up
CREATE TABLE spans (
    id String,
    organization_id String,
    trace_id String,
    span_id String,
    parent_span_id String DEFAULT '',
    trace_state String DEFAULT '',
    name LowCardinality (String),
    kind UInt8 DEFAULT 0,
    start_time DateTime64 (9),
    end_time DateTime64 (9),
    duration_ns Int64,
    status_code UInt8 DEFAULT 0,
    status_message String DEFAULT '',
    resource_attributes Map (String, String),
    scope_attributes Map (String, String),
    span_attributes Map (String, String),
    resource_schema_url String DEFAULT '',
    scope_name String DEFAULT '',
    scope_version String DEFAULT '',
    scope_schema_url String DEFAULT '',
    events_name Array (String),
    events_timestamp Array (DateTime64 (9)),
    events_attributes Array (Map (String, String)),
    links_trace_id Array (String),
    links_span_id Array (String),
    links_trace_state Array (String),
    links_attributes Array (Map (String, String)),
    service_name LowCardinality (String),
    deployment_environment LowCardinality (String)
) ENGINE = MergeTree
PARTITION BY
    toDate (start_time)
ORDER BY
    (
        organization_id,
        service_name,
        start_time,
        trace_id
    )
SETTINGS
    index_granularity = 8192;

ALTER TABLE spans ADD INDEX idx_trace_id trace_id TYPE bloom_filter GRANULARITY 4;

-- +goose Down
DROP TABLE IF EXISTS spans;
