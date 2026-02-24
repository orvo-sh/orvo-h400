-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE archive_jobs (
    id VARCHAR(32) PRIMARY KEY,
    signal VARCHAR(16) NOT NULL,
    state VARCHAR(16) NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_archive_jobs_signal_state ON archive_jobs (signal, state);
CREATE INDEX idx_archive_jobs_created_at ON archive_jobs (created_at DESC);

CREATE TABLE archive_objects (
    id VARCHAR(32) PRIMARY KEY,
    archive_job_id VARCHAR(32) REFERENCES archive_jobs (id) ON DELETE SET NULL,
    organization_id VARCHAR(32) NOT NULL,
    signal VARCHAR(16) NOT NULL,
    day DATE NOT NULL,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    object_size_bytes BIGINT NOT NULL DEFAULT 0,
    row_count BIGINT NOT NULL DEFAULT 0,
    checksum TEXT NOT NULL DEFAULT '',
    schema_version INT NOT NULL DEFAULT 1,
    expires_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, signal, day, object_key)
);

CREATE INDEX idx_archive_objects_lookup ON archive_objects (organization_id, signal, day);
CREATE INDEX idx_archive_objects_retention ON archive_objects (expires_at, deleted_at);

CREATE TABLE restore_jobs (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL,
    signal VARCHAR(16) NOT NULL,
    start_day DATE NOT NULL,
    end_day DATE NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    requested_by VARCHAR(32),
    total_items INT NOT NULL DEFAULT 0,
    completed_items INT NOT NULL DEFAULT 0,
    total_bytes BIGINT NOT NULL DEFAULT 0,
    done_bytes BIGINT NOT NULL DEFAULT 0,
    estimated_seconds INT NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_restore_jobs_org_signal ON restore_jobs (organization_id, signal, created_at DESC);
CREATE INDEX idx_restore_jobs_state ON restore_jobs (state, created_at DESC);

CREATE TABLE restore_job_items (
    id VARCHAR(32) PRIMARY KEY,
    restore_job_id VARCHAR(32) NOT NULL REFERENCES restore_jobs (id) ON DELETE CASCADE,
    organization_id VARCHAR(32) NOT NULL,
    signal VARCHAR(16) NOT NULL,
    day DATE NOT NULL,
    object_key TEXT NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    object_size_bytes BIGINT NOT NULL DEFAULT 0,
    restored_rows BIGINT NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    UNIQUE (restore_job_id, day, object_key)
);

CREATE INDEX idx_restore_job_items_state ON restore_job_items (restore_job_id, state);

CREATE TABLE restored_coverage (
    organization_id VARCHAR(32) NOT NULL,
    signal VARCHAR(16) NOT NULL,
    day DATE NOT NULL,
    restore_job_id VARCHAR(32) REFERENCES restore_jobs (id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, signal, day)
);

CREATE INDEX idx_restored_coverage_expiry ON restored_coverage (expires_at);

CREATE TABLE worker_heartbeats (
    worker_name VARCHAR(128) PRIMARY KEY,
    instance_id VARCHAR(128) NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE logs_hot (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    observed_timestamp TIMESTAMPTZ NOT NULL,
    severity_number INT NOT NULL,
    severity_text TEXT NOT NULL,
    body TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    trace_flags BIGINT NOT NULL DEFAULT 0,
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    resource_schema_url TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    scope_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_schema_url TEXT NOT NULL DEFAULT '',
    log_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE ("timestamp");

CREATE TABLE logs_restored (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    observed_timestamp TIMESTAMPTZ NOT NULL,
    severity_number INT NOT NULL,
    severity_text TEXT NOT NULL,
    body TEXT NOT NULL,
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    trace_flags BIGINT NOT NULL DEFAULT 0,
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    resource_schema_url TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    scope_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_schema_url TEXT NOT NULL DEFAULT '',
    log_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    restore_job_id VARCHAR(32),
    restored_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE ("timestamp");

CREATE TABLE traces_hot (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    parent_span_id TEXT NOT NULL DEFAULT '',
    trace_state TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    kind INT NOT NULL DEFAULT 0,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_ns BIGINT NOT NULL DEFAULT 0,
    status_code INT NOT NULL DEFAULT 0,
    status_message TEXT NOT NULL DEFAULT '',
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    span_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    resource_schema_url TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    scope_schema_url TEXT NOT NULL DEFAULT '',
    events JSONB NOT NULL DEFAULT '[]'::JSONB,
    links JSONB NOT NULL DEFAULT '[]'::JSONB,
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (start_time);

CREATE TABLE traces_restored (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    parent_span_id TEXT NOT NULL DEFAULT '',
    trace_state TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    kind INT NOT NULL DEFAULT 0,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_ns BIGINT NOT NULL DEFAULT 0,
    status_code INT NOT NULL DEFAULT 0,
    status_message TEXT NOT NULL DEFAULT '',
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    span_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    resource_schema_url TEXT NOT NULL DEFAULT '',
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    scope_schema_url TEXT NOT NULL DEFAULT '',
    events JSONB NOT NULL DEFAULT '[]'::JSONB,
    links JSONB NOT NULL DEFAULT '[]'::JSONB,
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    restore_job_id VARCHAR(32),
    restored_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (start_time);

CREATE TABLE metrics_hot (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    metric_name TEXT NOT NULL,
    metric_type INT NOT NULL,
    metric_unit TEXT NOT NULL DEFAULT '',
    metric_description TEXT NOT NULL DEFAULT '',
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    start_time TIMESTAMPTZ NOT NULL,
    "time" TIMESTAMPTZ NOT NULL,
    value_int BIGINT,
    value_double DOUBLE PRECISION,
    aggregation_temporality INT NOT NULL DEFAULT 0,
    is_monotonic BOOLEAN NOT NULL DEFAULT FALSE,
    histogram_count BIGINT,
    histogram_sum DOUBLE PRECISION,
    histogram_min DOUBLE PRECISION,
    histogram_max DOUBLE PRECISION,
    histogram_bucket_counts BIGINT[] NOT NULL DEFAULT '{}',
    histogram_explicit_bounds DOUBLE PRECISION[] NOT NULL DEFAULT '{}',
    exemplars JSONB NOT NULL DEFAULT '[]'::JSONB,
    flags BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE ("time");

CREATE TABLE metrics_restored (
    id VARCHAR(32) NOT NULL,
    organization_id VARCHAR(32) NOT NULL,
    metric_name TEXT NOT NULL,
    metric_type INT NOT NULL,
    metric_unit TEXT NOT NULL DEFAULT '',
    metric_description TEXT NOT NULL DEFAULT '',
    service_name TEXT NOT NULL DEFAULT '',
    deployment_environment TEXT NOT NULL DEFAULT '',
    resource_attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    scope_name TEXT NOT NULL DEFAULT '',
    scope_version TEXT NOT NULL DEFAULT '',
    attributes JSONB NOT NULL DEFAULT '{}'::JSONB,
    start_time TIMESTAMPTZ NOT NULL,
    "time" TIMESTAMPTZ NOT NULL,
    value_int BIGINT,
    value_double DOUBLE PRECISION,
    aggregation_temporality INT NOT NULL DEFAULT 0,
    is_monotonic BOOLEAN NOT NULL DEFAULT FALSE,
    histogram_count BIGINT,
    histogram_sum DOUBLE PRECISION,
    histogram_min DOUBLE PRECISION,
    histogram_max DOUBLE PRECISION,
    histogram_bucket_counts BIGINT[] NOT NULL DEFAULT '{}',
    histogram_explicit_bounds DOUBLE PRECISION[] NOT NULL DEFAULT '{}',
    exemplars JSONB NOT NULL DEFAULT '[]'::JSONB,
    flags BIGINT NOT NULL DEFAULT 0,
    restore_job_id VARCHAR(32),
    restored_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE ("time");

CREATE INDEX idx_logs_hot_org_time ON logs_hot (organization_id, "timestamp" DESC);
CREATE INDEX idx_logs_hot_org_service_time ON logs_hot (organization_id, service_name, "timestamp" DESC);
CREATE INDEX idx_logs_hot_trace_id ON logs_hot (organization_id, trace_id);
CREATE INDEX idx_logs_hot_span_id ON logs_hot (organization_id, span_id);
CREATE INDEX idx_logs_hot_severity_time ON logs_hot (organization_id, severity_number, "timestamp" DESC);
CREATE INDEX idx_logs_hot_body_trgm ON logs_hot USING GIN (body gin_trgm_ops);
CREATE INDEX idx_logs_hot_resource_attrs ON logs_hot USING GIN (resource_attributes);
CREATE INDEX idx_logs_hot_scope_attrs ON logs_hot USING GIN (scope_attributes);
CREATE INDEX idx_logs_hot_log_attrs ON logs_hot USING GIN (log_attributes);

CREATE INDEX idx_logs_restored_org_time ON logs_restored (organization_id, "timestamp" DESC);
CREATE INDEX idx_logs_restored_org_service_time ON logs_restored (organization_id, service_name, "timestamp" DESC);
CREATE INDEX idx_logs_restored_trace_id ON logs_restored (organization_id, trace_id);
CREATE INDEX idx_logs_restored_span_id ON logs_restored (organization_id, span_id);

CREATE INDEX idx_traces_hot_org_start ON traces_hot (organization_id, start_time DESC);
CREATE INDEX idx_traces_hot_org_service_start ON traces_hot (organization_id, service_name, start_time DESC);
CREATE INDEX idx_traces_hot_org_trace_id ON traces_hot (organization_id, trace_id);
CREATE INDEX idx_traces_hot_org_span_id ON traces_hot (organization_id, span_id);
CREATE INDEX idx_traces_hot_org_parent_span_id ON traces_hot (organization_id, parent_span_id);
CREATE INDEX idx_traces_hot_name_trgm ON traces_hot USING GIN (name gin_trgm_ops);
CREATE INDEX idx_traces_hot_resource_attrs ON traces_hot USING GIN (resource_attributes);
CREATE INDEX idx_traces_hot_scope_attrs ON traces_hot USING GIN (scope_attributes);
CREATE INDEX idx_traces_hot_span_attrs ON traces_hot USING GIN (span_attributes);

CREATE INDEX idx_traces_restored_org_start ON traces_restored (organization_id, start_time DESC);
CREATE INDEX idx_traces_restored_org_service_start ON traces_restored (organization_id, service_name, start_time DESC);
CREATE INDEX idx_traces_restored_org_trace_id ON traces_restored (organization_id, trace_id);

CREATE INDEX idx_metrics_hot_org_time ON metrics_hot (organization_id, "time" DESC);
CREATE INDEX idx_metrics_hot_org_metric_time ON metrics_hot (organization_id, metric_name, "time" DESC);
CREATE INDEX idx_metrics_hot_org_service_time ON metrics_hot (organization_id, service_name, "time" DESC);
CREATE INDEX idx_metrics_hot_attrs ON metrics_hot USING GIN (attributes);
CREATE INDEX idx_metrics_hot_resource_attrs ON metrics_hot USING GIN (resource_attributes);

CREATE INDEX idx_metrics_restored_org_time ON metrics_restored (organization_id, "time" DESC);
CREATE INDEX idx_metrics_restored_org_metric_time ON metrics_restored (organization_id, metric_name, "time" DESC);
CREATE INDEX idx_metrics_restored_org_service_time ON metrics_restored (organization_id, service_name, "time" DESC);

CREATE TABLE logs_hot_default PARTITION OF logs_hot DEFAULT;
CREATE TABLE logs_restored_default PARTITION OF logs_restored DEFAULT;
CREATE TABLE traces_hot_default PARTITION OF traces_hot DEFAULT;
CREATE TABLE traces_restored_default PARTITION OF traces_restored DEFAULT;
CREATE TABLE metrics_hot_default PARTITION OF metrics_hot DEFAULT;
CREATE TABLE metrics_restored_default PARTITION OF metrics_restored DEFAULT;

DO $$
DECLARE
    day DATE;
BEGIN
    FOR day IN
        SELECT generate_series(
            (CURRENT_DATE - INTERVAL '30 days')::DATE,
            (CURRENT_DATE + INTERVAL '7 days')::DATE,
            INTERVAL '1 day'
        )::DATE
    LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS logs_hot_p%s PARTITION OF logs_hot FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS logs_restored_p%s PARTITION OF logs_restored FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS traces_hot_p%s PARTITION OF traces_hot FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS traces_restored_p%s PARTITION OF traces_restored FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS metrics_hot_p%s PARTITION OF metrics_hot FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS metrics_restored_p%s PARTITION OF metrics_restored FOR VALUES FROM (%L) TO (%L)',
            to_char(day, 'YYYYMMDD'),
            day,
            day + INTERVAL '1 day'
        );
    END LOOP;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS metrics_restored CASCADE;
DROP TABLE IF EXISTS metrics_hot CASCADE;
DROP TABLE IF EXISTS traces_restored CASCADE;
DROP TABLE IF EXISTS traces_hot CASCADE;
DROP TABLE IF EXISTS logs_restored CASCADE;
DROP TABLE IF EXISTS logs_hot CASCADE;
DROP TABLE IF EXISTS worker_heartbeats;
DROP TABLE IF EXISTS restored_coverage;
DROP TABLE IF EXISTS restore_job_items;
DROP TABLE IF EXISTS restore_jobs;
DROP TABLE IF EXISTS archive_objects;
DROP TABLE IF EXISTS archive_jobs;

-- +goose StatementEnd
