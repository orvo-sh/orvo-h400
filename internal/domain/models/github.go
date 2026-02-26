package models

import "time"

type GithubInstallation struct {
	ID                   string    `json:"id"`
	OrganizationID       string    `json:"organization_id"`
	GithubInstallationID int64     `json:"github_installation_id"`
	AccountID            int64     `json:"account_id"`
	AccountLogin         string    `json:"account_login"`
	AccountType          string    `json:"account_type"`
	Active               bool      `json:"active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type GithubRepository struct {
	ID                 string    `json:"id"`
	OrganizationID     string    `json:"organization_id"`
	InstallationID     string    `json:"installation_id"`
	GithubRepositoryID int64     `json:"github_repository_id"`
	FullName           string    `json:"full_name"`
	DefaultBranch      string    `json:"default_branch"`
	Private            bool      `json:"private"`
	Archived           bool      `json:"archived"`
	Enabled            bool      `json:"enabled"`
	LastSyncedAt       time.Time `json:"last_synced_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
