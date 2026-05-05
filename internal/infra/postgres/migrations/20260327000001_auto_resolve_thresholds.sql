-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auto_resolve_thresholds (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    metric_name TEXT NOT NULL DEFAULT 'derived.errors.rate',
    threshold_value DOUBLE PRECISION NOT NULL CHECK (threshold_value > 0),
    lookback_window_seconds INT NOT NULL CHECK (lookback_window_seconds > 0),
    cooldown_seconds INT NOT NULL CHECK (cooldown_seconds > 0),
    quorum INT NOT NULL CHECK (quorum > 0),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at TIMESTAMPTZ,
    created_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, service_name)
);

CREATE INDEX IF NOT EXISTS idx_auto_resolve_thresholds_org_enabled
    ON auto_resolve_thresholds (organization_id, enabled, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_auto_resolve_thresholds_service
    ON auto_resolve_thresholds (organization_id, service_name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_auto_resolve_thresholds_service;
DROP INDEX IF EXISTS idx_auto_resolve_thresholds_org_enabled;
DROP TABLE IF EXISTS auto_resolve_thresholds;
-- +goose StatementEnd
