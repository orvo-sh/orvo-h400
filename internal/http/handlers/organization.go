package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/organizationservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type OrganizationHandler struct {
	orgService  organizationservice.Service
	authService authservice.Service
}

func NewOrganizationHandler(orgService organizationservice.Service, authService authservice.Service) *OrganizationHandler {
	return &OrganizationHandler{
		orgService:  orgService,
		authService: authService,
	}
}

func (h *OrganizationHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "create-organization",
		Method:      http.MethodPost,
		Path:        "/organizations",
		Tags:        []string{"organizations"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createOrganization)

	huma.Register(api, huma.Operation{
		OperationID: "list-organizations",
		Method:      http.MethodGet,
		Path:        "/organizations",
		Tags:        []string{"organizations"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listOrganizations)
}

func (h *OrganizationHandler) createOrganization(ctx context.Context, input *dto.CreateOrganizationInput) (*dto.CreateOrganizationOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)

	organization, err := h.orgService.CreateOrganization(ctx, organizationservice.CreateOrganizationInput{
		Name:          input.Body.Name,
		Logo:          input.Body.Logo,
		CreatorUserID: session.UserID,
	})
	if err != nil {
		return nil, err
	}

	if input.Body.SetAsActiveOrganization {
		if err := h.authService.SetActiveOrganization(ctx, authservice.SetActiveOrganizationInput{
			SessionToken:   session.Token,
			OrganizationID: &organization.ID,
		}); err != nil {
			return nil, err
		}
	}

	return &dto.CreateOrganizationOutput{
		Body: struct {
			ID string `json:"id"`
		}{
			ID: organization.ID,
		},
	}, nil
}

func (h *OrganizationHandler) listOrganizations(ctx context.Context, input *struct{}) (*dto.ListOrganizationsOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)

	items, err := h.orgService.ListOrganizations(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	organizations := make([]dto.Organization, len(items))
	for i, item := range items {
		organizations[i] = dto.Organization{
			ID:        item.ID,
			Name:      item.Name,
			Slug:      item.Slug,
			Logo:      item.Logo,
			Role:      item.Role,
			CreatedAt: item.CreatedAt,
		}
	}

	return &dto.ListOrganizationsOutput{
		Body: struct {
			Organizations []dto.Organization `json:"organizations"`
		}{
			Organizations: organizations,
		},
	}, nil
}
