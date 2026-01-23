-- +goose Up
CREATE TABLE logs (
    id String DEFAULT,
    timestamp DateTime64 (3) DEFAULT now(),
    level LowCardinality (String),
    message LowCardinality (String),
    service LowCardinality (String),
    environment LowCardinality (String),
    organization_id String,
    trace_id String DEFAULT '',
    span_id String DEFAULT '',
    parent_id String DEFAULT '',
    attributes JSON DEFAULT '{}'
) ENGINE = MergeTree
PARTITION BY
    toDate (timestamp)
ORDER BY (timestamp, service, level) SETTINGS index_granularity = 8192;