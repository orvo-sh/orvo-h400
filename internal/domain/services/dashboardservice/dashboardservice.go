package dashboardservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	CreateDashboard(ctx context.Context, input CreateDashboardInput) (*models.Dashboard, apperr.Error)
	GetDashboard(ctx context.Context, orgID string, dashboardID string) (*models.Dashboard, apperr.Error)
	ListDashboards(ctx context.Context, orgID string) ([]models.Dashboard, apperr.Error)
	UpdateDashboard(ctx context.Context, input UpdateDashboardInput) (*models.Dashboard, apperr.Error)
	DeleteDashboard(ctx context.Context, orgID string, dashboardID string) apperr.Error
}

type service struct {
	pg     *postgres.DB
	logger *slog.Logger
}

func New(pg *postgres.DB, logger *slog.Logger) Service {
	return &service{
		pg:     pg,
		logger: logger,
	}
}

type CreateDashboardInput struct {
	OrganizationID string
	Name           string
	Description    string
	Panels         []models.DashboardPanel
	Layout         []models.DashboardLayout
	CreatedBy      string
}

type UpdateDashboardInput struct {
	OrganizationID string
	DashboardID    string
	Name           string
	Description    string
	Panels         []models.DashboardPanel
	Layout         []models.DashboardLayout
}
