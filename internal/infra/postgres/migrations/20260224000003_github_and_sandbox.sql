-- +goose Up
-- +goose StatementBegin

CREATE TABLE github_install_states (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    state_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_github_install_states_org_user ON github_install_states (organization_id, user_id);
CREATE INDEX idx_github_install_states_expires ON github_install_states (expires_at);

CREATE TABLE github_installations (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    github_installation_id BIGINT NOT NULL UNIQUE,
    account_id BIGINT NOT NULL DEFAULT 0,
    account_login TEXT NOT NULL DEFAULT '',
    account_type TEXT NOT NULL DEFAULT '',
    created_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_github_installations_org ON github_installations (organization_id);
CREATE INDEX idx_github_installations_active ON github_installations (active);

CREATE TABLE github_repositories (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    installation_id VARCHAR(32) NOT NULL REFERENCES github_installations (id) ON DELETE CASCADE,
    github_repository_id BIGINT NOT NULL,
    full_name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    private BOOLEAN NOT NULL DEFAULT TRUE,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, github_repository_id)
);

CREATE INDEX idx_github_repositories_org ON github_repositories (organization_id);
CREATE INDEX idx_github_repositories_installation ON github_repositories (installation_id);
CREATE INDEX idx_github_repositories_enabled ON github_repositories (organization_id, enabled);

CREATE TABLE sandbox_jobs (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    repository_id VARCHAR(32) NOT NULL REFERENCES github_repositories (id) ON DELETE RESTRICT,
    requested_by_user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    state VARCHAR(16) NOT NULL DEFAULT 'queued',
    runtime_type VARCHAR(32) NOT NULL DEFAULT '',
    sandbox_instance_id TEXT NOT NULL DEFAULT '',
    task_title TEXT NOT NULL DEFAULT '',
    commit_message TEXT NOT NULL DEFAULT '',
    base_branch TEXT NOT NULL DEFAULT '',
    branch_name TEXT NOT NULL DEFAULT '',
    draft_pr BOOLEAN NOT NULL DEFAULT TRUE,
    pull_request_number BIGINT,
    pull_request_url TEXT,
    cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sandbox_jobs_queue ON sandbox_jobs (state, created_at);
CREATE INDEX idx_sandbox_jobs_org ON sandbox_jobs (organization_id, created_at DESC);

CREATE TABLE sandbox_job_logs (
    id BIGSERIAL PRIMARY KEY,
    sandbox_job_id VARCHAR(32) NOT NULL REFERENCES sandbox_jobs (id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    stream VARCHAR(16) NOT NULL DEFAULT 'stdout',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (sandbox_job_id, seq)
);

CREATE INDEX idx_sandbox_job_logs_lookup ON sandbox_job_logs (sandbox_job_id, seq);

CREATE TABLE sandbox_job_commands (
    id VARCHAR(32) PRIMARY KEY,
    sandbox_job_id VARCHAR(32) NOT NULL REFERENCES sandbox_jobs (id) ON DELETE CASCADE,
    ordinal INT NOT NULL,
    command TEXT NOT NULL,
    exit_code INT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (sandbox_job_id, ordinal)
);

CREATE INDEX idx_sandbox_job_commands_job ON sandbox_job_commands (sandbox_job_id, ordinal);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sandbox_job_commands;
DROP TABLE IF EXISTS sandbox_job_logs;
DROP TABLE IF EXISTS sandbox_jobs;
DROP TABLE IF EXISTS github_repositories;
DROP TABLE IF EXISTS github_installations;
DROP TABLE IF EXISTS github_install_states;
-- +goose StatementEnd
