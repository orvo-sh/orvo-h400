-- +goose Up
-- +goose StatementBegin

CREATE TABLE service_remediation_mappings (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    repository_id VARCHAR(32) NOT NULL REFERENCES github_repositories (id) ON DELETE CASCADE,
    created_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    updated_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_service_remediation_mappings_org ON service_remediation_mappings (organization_id);
CREATE INDEX idx_service_remediation_mappings_repo ON service_remediation_mappings (repository_id);
CREATE UNIQUE INDEX idx_service_remediation_mappings_org_service_lower ON service_remediation_mappings (organization_id, LOWER(service_name));

ALTER TABLE sandbox_jobs
    ADD COLUMN mode VARCHAR(32) NOT NULL DEFAULT 'manual',
    ADD COLUMN incident_context_json TEXT NOT NULL DEFAULT '',
    ADD COLUMN incident_prompt TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_sandbox_jobs_org_mode_created_at ON sandbox_jobs (organization_id, mode, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sandbox_jobs_org_mode_created_at;

ALTER TABLE sandbox_jobs
    DROP COLUMN IF EXISTS incident_prompt,
    DROP COLUMN IF EXISTS incident_context_json,
    DROP COLUMN IF EXISTS mode;

DROP INDEX IF EXISTS idx_service_remediation_mappings_repo;
DROP INDEX IF EXISTS idx_service_remediation_mappings_org;
DROP INDEX IF EXISTS idx_service_remediation_mappings_org_service_lower;
DROP TABLE IF EXISTS service_remediation_mappings;
-- +goose StatementEnd
