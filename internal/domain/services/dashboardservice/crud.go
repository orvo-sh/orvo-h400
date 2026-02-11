package dashboardservice

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	db "github.com/orvo-sh/orvo/internal/infra/postgres/db"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

func (s *service) CreateDashboard(ctx context.Context, input CreateDashboardInput) (*models.Dashboard, apperr.Error) {
	s.logger.InfoContext(ctx, "CreateDashboard",
		slog.String("organization_id", input.OrganizationID),
		slog.String("name", input.Name),
	)

	panelsJSON, err := json.Marshal(input.Panels)
	if err != nil {
		s.logger.ErrorContext(ctx, "CreateDashboard: failed to marshal panels", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	layoutJSON, err := json.Marshal(input.Layout)
	if err != nil {
		s.logger.ErrorContext(ctx, "CreateDashboard: failed to marshal layout", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	row, err := s.pg.Queries.CreateDashboard(ctx, db.CreateDashboardParams{
		ID:             util.GenerateID("dash"),
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Description:    input.Description,
		Panels:         panelsJSON,
		Layout:         layoutJSON,
		CreatedBy:      input.CreatedBy,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "CreateDashboard: insert failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return dbDashboardToModel(row), nil
}

func (s *service) GetDashboard(ctx context.Context, orgID string, dashboardID string) (*models.Dashboard, apperr.Error) {
	row, err := s.pg.Queries.GetDashboardByID(ctx, db.GetDashboardByIDParams{
		ID:             dashboardID,
		OrganizationID: orgID,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrNotFound
		}
		s.logger.ErrorContext(ctx, "GetDashboard: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return dbDashboardToModel(row), nil
}

func (s *service) ListDashboards(ctx context.Context, orgID string) ([]models.Dashboard, apperr.Error) {
	rows, err := s.pg.Queries.ListDashboardsByOrganizationID(ctx, orgID)
	if err != nil {
		s.logger.ErrorContext(ctx, "ListDashboards: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	dashboards := make([]models.Dashboard, len(rows))
	for i, row := range rows {
		dashboards[i] = *dbDashboardToModel(row)
	}

	return dashboards, nil
}

func (s *service) UpdateDashboard(ctx context.Context, input UpdateDashboardInput) (*models.Dashboard, apperr.Error) {
	s.logger.InfoContext(ctx, "UpdateDashboard",
		slog.String("organization_id", input.OrganizationID),
		slog.String("dashboard_id", input.DashboardID),
	)

	panelsJSON, err := json.Marshal(input.Panels)
	if err != nil {
		s.logger.ErrorContext(ctx, "UpdateDashboard: failed to marshal panels", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	layoutJSON, err := json.Marshal(input.Layout)
	if err != nil {
		s.logger.ErrorContext(ctx, "UpdateDashboard: failed to marshal layout", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	row, err := s.pg.Queries.UpdateDashboard(ctx, db.UpdateDashboardParams{
		ID:             input.DashboardID,
		OrganizationID: input.OrganizationID,
		Name:           input.Name,
		Description:    input.Description,
		Panels:         panelsJSON,
		Layout:         layoutJSON,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrNotFound
		}
		s.logger.ErrorContext(ctx, "UpdateDashboard: update failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return dbDashboardToModel(row), nil
}

func (s *service) DeleteDashboard(ctx context.Context, orgID string, dashboardID string) apperr.Error {
	s.logger.InfoContext(ctx, "DeleteDashboard",
		slog.String("organization_id", orgID),
		slog.String("dashboard_id", dashboardID),
	)

	if err := s.pg.Queries.DeleteDashboard(ctx, db.DeleteDashboardParams{
		ID:             dashboardID,
		OrganizationID: orgID,
	}); err != nil {
		s.logger.ErrorContext(ctx, "DeleteDashboard: delete failed", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}

func dbDashboardToModel(row db.Dashboard) *models.Dashboard {
	var panels []models.DashboardPanel
	_ = json.Unmarshal(row.Panels, &panels)
	if panels == nil {
		panels = []models.DashboardPanel{}
	}

	var layout []models.DashboardLayout
	_ = json.Unmarshal(row.Layout, &layout)
	if layout == nil {
		layout = []models.DashboardLayout{}
	}

	createdAt := time.Time{}
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}
	updatedAt := time.Time{}
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time
	}

	return &models.Dashboard{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		Description:    row.Description,
		Panels:         panels,
		Layout:         layout,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}
