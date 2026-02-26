package dto

import "github.com/orvo-sh/orvo/internal/domain/models"

type CreateGithubInstallURLInput struct {
	OrganizationID string `path:"organization_id"`
}

type CreateGithubInstallURLOutput struct {
	Body struct {
		URL string `json:"url"`
	}
}

type ListGithubInstallationsInput struct {
	OrganizationID string `path:"organization_id"`
}

type ListGithubInstallationsOutput struct {
	Body struct {
		Installations []models.GithubInstallation `json:"installations"`
	}
}

type ListGithubRepositoriesInput struct {
	OrganizationID string `path:"organization_id"`
}

type ListGithubRepositoriesOutput struct {
	Body struct {
		Repositories []models.GithubRepository `json:"repositories"`
	}
}

type SetGithubRepositoryEnabledInput struct {
	OrganizationID string `path:"organization_id"`
	RepositoryID   string `path:"repository_id"`
	Body           struct {
		Enabled bool `json:"enabled"`
	}
}

type GithubSetupCallbackInput struct {
	InstallationID int64  `query:"installation_id"`
	SetupAction    string `query:"setup_action"`
	State          string `query:"state"`
}

type GithubSetupCallbackOutput struct {
	Body struct {
		RedirectURL string `json:"redirect_url"`
	}
}
