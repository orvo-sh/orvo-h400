-- +goose Up
-- +goose StatementBegin

-- Users table
CREATE TABLE users (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    name VARCHAR(255) NOT NULL,
    image VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Accounts table (for OAuth providers, credentials, etc.)
CREATE TABLE accounts (
    id VARCHAR(32) PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- 'credential', 'google', 'github', etc.
    provider_account_id VARCHAR(255) NOT NULL, -- email for credential, OAuth ID for OAuth
    password_hash VARCHAR(255), -- only for credential provider
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_account_id)
);

CREATE INDEX idx_accounts_user_id ON accounts (user_id);

-- Organizations table
CREATE TABLE organizations (
    id VARCHAR(32) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    logo VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organizations_slug ON organizations (slug);

-- Organization members table (join table between users and organizations)
CREATE TABLE organization_members (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member', -- 'owner', 'admin', 'member'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX idx_org_members_org_id ON organization_members (organization_id);

CREATE INDEX idx_org_members_user_id ON organization_members (user_id);

-- Organization invitations table
CREATE TABLE organization_invitations (
    id VARCHAR(32) PRIMARY KEY,
    organization_id VARCHAR(32) NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    invited_by_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'accepted', 'rejected', 'cancelled'
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_org_invitations_org_id ON organization_invitations (organization_id);

CREATE INDEX idx_org_invitations_email ON organization_invitations (email);

-- Sessions table (inspired by better-auth)
CREATE TABLE sessions (
    id VARCHAR(32) PRIMARY KEY,
    token VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(32) NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    active_organization_id VARCHAR(32) REFERENCES organizations (id) ON DELETE SET NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_token ON sessions (token);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);

CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;

DROP TABLE IF EXISTS organization_invitations;

DROP TABLE IF EXISTS organization_members;

DROP TABLE IF EXISTS organizations;

DROP TABLE IF EXISTS accounts;

DROP TABLE IF EXISTS users;
-- +goose StatementEnd