package dto

import "github.com/orvo-sh/orvo/internal/domain/models"

type ListServiceRemediationMappingsInput struct {
	OrganizationID string `path:"organization_id"`
}

type ServiceRemediationMapping struct {
	ServiceName        string `json:"service_name"`
	RepositoryID       string `json:"repository_id"`
	RepositoryFullName string `json:"repository_full_name"`
	UpdatedAt          string `json:"updated_at"`
}

type ListServiceRemediationMappingsOutput struct {
	Body struct {
		Mappings []models.ServiceRemediationMapping `json:"mappings"`
	}
}

type UpsertServiceRemediationMappingInput struct {
	OrganizationID string `path:"organization_id"`
	ServiceName    string `path:"service_name"`
	Body           struct {
		RepositoryID string `json:"repository_id" minLength:"1"`
	}
}

type UpsertServiceRemediationMappingOutput struct {
	Body models.ServiceRemediationMapping
}

type DeleteServiceRemediationMappingInput struct {
	OrganizationID string `path:"organization_id"`
	ServiceName    string `path:"service_name"`
}

type GetLogAutoResolvePreviewInput struct {
	OrganizationID string `path:"organization_id"`
	LogID          string `path:"log_id"`
}

type GetLogAutoResolvePreviewOutput struct {
	Body models.AutoResolvePreview
}

type RunLogAutoResolveInput struct {
	OrganizationID string `path:"organization_id"`
	LogID          string `path:"log_id"`
}

type RunLogAutoResolveOutput struct {
	Body struct {
		JobID string `json:"job_id"`
	}
}
