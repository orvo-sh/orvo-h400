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

type ListAutoResolveThresholdsInput struct {
	OrganizationID string `path:"organization_id"`
}

type AutoResolveThreshold struct {
	ID              string  `json:"id"`
	ServiceName     string  `json:"service_name"`
	MetricName      string  `json:"metric_name"`
	ThresholdValue  float64 `json:"threshold_value"`
	LookbackWindow  string  `json:"lookback_window"`
	Cooldown        string  `json:"cooldown"`
	Quorum          int     `json:"quorum"`
	Enabled         bool    `json:"enabled"`
	LastTriggeredAt string  `json:"last_triggered_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type ListAutoResolveThresholdsOutput struct {
	Body struct {
		Thresholds []AutoResolveThreshold `json:"thresholds"`
	}
}

type UpsertAutoResolveThresholdInput struct {
	OrganizationID string `path:"organization_id"`
	ServiceName    string `path:"service_name"`
	Body           struct {
		ThresholdValue float64 `json:"threshold_value"`
		LookbackWindow string  `json:"lookback_window"`
		Cooldown       string  `json:"cooldown"`
		Quorum         int     `json:"quorum"`
		Enabled        bool    `json:"enabled"`
	}
}

type UpsertAutoResolveThresholdOutput struct {
	Body AutoResolveThreshold
}

type DeleteAutoResolveThresholdInput struct {
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
