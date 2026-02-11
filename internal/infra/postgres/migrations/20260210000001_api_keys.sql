-- +goose Up
-- +goose StatementBegin

CREATE TABLE api_keys (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_org ON api_keys (organization_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys (key_hash);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS api_keys;

-- +goose StatementEnd
