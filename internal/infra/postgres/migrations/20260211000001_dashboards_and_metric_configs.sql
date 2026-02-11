-- +goose Up
-- +goose StatementBegin

-- Dashboards table: stores user-created custom dashboards.
-- Panels and layout are stored as JSONB since they are flexible structures.
CREATE TABLE dashboards (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    panels JSONB NOT NULL DEFAULT '[]',
    layout JSONB NOT NULL DEFAULT '[]',
    created_by VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dashboards_org ON dashboards (organization_id);
CREATE INDEX idx_dashboards_created_by ON dashboards (created_by);

-- Metric configurations table: stores user-configurable settings for derived/computed metrics.
-- Examples: Apdex threshold (T), error budget target (99.9%), health score weights.
CREATE TABLE metric_configurations (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    metric_name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, metric_name)
);

CREATE INDEX idx_metric_configs_org ON metric_configurations (organization_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS metric_configurations;
DROP TABLE IF EXISTS dashboards;
-- +goose StatementEnd
