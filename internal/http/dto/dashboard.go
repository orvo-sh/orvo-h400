package dto

import (
	"github.com/orvo-sh/orvo/internal/domain/models"
)

// --- Dashboard CRUD ---

type CreateDashboardInput struct {
	OrganizationID string `path:"organization_id"`
	Body           struct {
		Name        string                   `json:"name" doc:"Dashboard name" required:"true"`
		Description string                   `json:"description" doc:"Dashboard description" required:"false"`
		Panels      []models.DashboardPanel  `json:"panels" doc:"Dashboard panels" required:"false"`
		Layout      []models.DashboardLayout `json:"layout" doc:"Panel layout positions" required:"false"`
	}
}

type CreateDashboardOutput struct {
	Body models.Dashboard
}

type GetDashboardInput struct {
	OrganizationID string `path:"organization_id"`
	DashboardID    string `path:"dashboard_id"`
}

type GetDashboardOutput struct {
	Body models.Dashboard
}

type ListDashboardsInput struct {
	OrganizationID string `path:"organization_id"`
}

type ListDashboardsOutput struct {
	Body struct {
		Dashboards []models.Dashboard `json:"dashboards"`
	}
}

type UpdateDashboardInput struct {
	OrganizationID string `path:"organization_id"`
	DashboardID    string `path:"dashboard_id"`
	Body           struct {
		Name        string                   `json:"name" doc:"Dashboard name" required:"true"`
		Description string                   `json:"description" doc:"Dashboard description" required:"false"`
		Panels      []models.DashboardPanel  `json:"panels" doc:"Dashboard panels" required:"false"`
		Layout      []models.DashboardLayout `json:"layout" doc:"Panel layout positions" required:"false"`
	}
}

type UpdateDashboardOutput struct {
	Body models.Dashboard
}

type DeleteDashboardInput struct {
	OrganizationID string `path:"organization_id"`
	DashboardID    string `path:"dashboard_id"`
}

type DeleteDashboardOutput struct {
	Body Empty
}
