package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/dashboardservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type DashboardHandler struct {
	dashboardService dashboardservice.Service
	authService      authservice.Service
}

func NewDashboardHandler(dashboardService dashboardservice.Service, authService authservice.Service) *DashboardHandler {
	return &DashboardHandler{
		dashboardService: dashboardService,
		authService:      authService,
	}
}

func (h *DashboardHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "create-dashboard",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/dashboards",
		Tags:        []string{"dashboards"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createDashboard)

	huma.Register(api, huma.Operation{
		OperationID: "get-dashboard",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/dashboards/{dashboard_id}",
		Tags:        []string{"dashboards"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getDashboard)

	huma.Register(api, huma.Operation{
		OperationID: "list-dashboards",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/dashboards",
		Tags:        []string{"dashboards"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listDashboards)

	huma.Register(api, huma.Operation{
		OperationID: "update-dashboard",
		Method:      http.MethodPut,
		Path:        "/organizations/{organization_id}/dashboards/{dashboard_id}",
		Tags:        []string{"dashboards"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.updateDashboard)

	huma.Register(api, huma.Operation{
		OperationID: "delete-dashboard",
		Method:      http.MethodDelete,
		Path:        "/organizations/{organization_id}/dashboards/{dashboard_id}",
		Tags:        []string{"dashboards"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.deleteDashboard)
}

func (h *DashboardHandler) createDashboard(ctx context.Context, input *dto.CreateDashboardInput) (*dto.CreateDashboardOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)

	result, err := h.dashboardService.CreateDashboard(ctx, dashboardservice.CreateDashboardInput{
		OrganizationID: input.OrganizationID,
		Name:           input.Body.Name,
		Description:    input.Body.Description,
		Panels:         input.Body.Panels,
		Layout:         input.Body.Layout,
		CreatedBy:      session.UserID,
	})
	if err != nil {
		return nil, err
	}

	return &dto.CreateDashboardOutput{Body: *result}, nil
}

func (h *DashboardHandler) getDashboard(ctx context.Context, input *dto.GetDashboardInput) (*dto.GetDashboardOutput, error) {
	result, err := h.dashboardService.GetDashboard(ctx, input.OrganizationID, input.DashboardID)
	if err != nil {
		return nil, err
	}

	return &dto.GetDashboardOutput{Body: *result}, nil
}

func (h *DashboardHandler) listDashboards(ctx context.Context, input *dto.ListDashboardsInput) (*dto.ListDashboardsOutput, error) {
	results, err := h.dashboardService.ListDashboards(ctx, input.OrganizationID)
	if err != nil {
		return nil, err
	}

	out := &dto.ListDashboardsOutput{}
	out.Body.Dashboards = results
	return out, nil
}

func (h *DashboardHandler) updateDashboard(ctx context.Context, input *dto.UpdateDashboardInput) (*dto.UpdateDashboardOutput, error) {
	result, err := h.dashboardService.UpdateDashboard(ctx, dashboardservice.UpdateDashboardInput{
		OrganizationID: input.OrganizationID,
		DashboardID:    input.DashboardID,
		Name:           input.Body.Name,
		Description:    input.Body.Description,
		Panels:         input.Body.Panels,
		Layout:         input.Body.Layout,
	})
	if err != nil {
		return nil, err
	}

	return &dto.UpdateDashboardOutput{Body: *result}, nil
}

func (h *DashboardHandler) deleteDashboard(ctx context.Context, input *dto.DeleteDashboardInput) (*dto.DeleteDashboardOutput, error) {
	if err := h.dashboardService.DeleteDashboard(ctx, input.OrganizationID, input.DashboardID); err != nil {
		return nil, err
	}

	return &dto.DeleteDashboardOutput{}, nil
}
