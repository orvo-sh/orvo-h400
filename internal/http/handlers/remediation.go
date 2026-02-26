package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/remediationservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/helpers"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type RemediationHandler struct {
	authService        authservice.Service
	remediationService remediationservice.Service
}

func NewRemediationHandler(authService authservice.Service, remediationService remediationservice.Service) *RemediationHandler {
	return &RemediationHandler{
		authService:        authService,
		remediationService: remediationService,
	}
}

func (h *RemediationHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "list-service-remediation-mappings",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/remediation/service-mappings",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listServiceMappings)

	huma.Register(api, huma.Operation{
		OperationID: "upsert-service-remediation-mapping",
		Method:      http.MethodPut,
		Path:        "/organizations/{organization_id}/remediation/service-mappings/{service_name}",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.upsertServiceMapping)

	huma.Register(api, huma.Operation{
		OperationID: "delete-service-remediation-mapping",
		Method:      http.MethodDelete,
		Path:        "/organizations/{organization_id}/remediation/service-mappings/{service_name}",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.deleteServiceMapping)

	huma.Register(api, huma.Operation{
		OperationID: "get-log-auto-resolve-preview",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/logs/{log_id}/auto-resolve/preview",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.previewLogAutoResolve)

	huma.Register(api, huma.Operation{
		OperationID: "run-log-auto-resolve",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/logs/{log_id}/auto-resolve",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.runLogAutoResolve)
}

func (h *RemediationHandler) listServiceMappings(ctx context.Context, input *dto.ListServiceRemediationMappingsInput) (*dto.ListServiceRemediationMappingsOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.authService.EnsureOrganizationMember(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	mappings, appErr := h.remediationService.ListMappings(ctx, input.OrganizationID)
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.ListServiceRemediationMappingsOutput{
		Body: struct {
			Mappings []models.ServiceRemediationMapping `json:"mappings"`
		}{
			Mappings: mappings,
		},
	}, nil
}

func (h *RemediationHandler) upsertServiceMapping(ctx context.Context, input *dto.UpsertServiceRemediationMappingInput) (*dto.UpsertServiceRemediationMappingOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	result, appErr := h.remediationService.UpsertMapping(ctx, remediationservice.UpsertMappingInput{
		OrganizationID: input.OrganizationID,
		ServiceName:    strings.TrimSpace(input.ServiceName),
		RepositoryID:   input.Body.RepositoryID,
		UserID:         session.UserID,
	})
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.UpsertServiceRemediationMappingOutput{
		Body: *result,
	}, nil
}

func (h *RemediationHandler) deleteServiceMapping(ctx context.Context, input *dto.DeleteServiceRemediationMappingInput) (*dto.Empty, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	if appErr := h.remediationService.DeleteMapping(ctx, input.OrganizationID, strings.TrimSpace(input.ServiceName)); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return nil, nil
}

func (h *RemediationHandler) previewLogAutoResolve(ctx context.Context, input *dto.GetLogAutoResolvePreviewInput) (*dto.GetLogAutoResolvePreviewOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.authService.EnsureOrganizationMember(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	preview, appErr := h.remediationService.PreviewAutoResolve(ctx, remediationservice.PreviewAutoResolveInput{
		OrganizationID: input.OrganizationID,
		LogID:          input.LogID,
	})
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.GetLogAutoResolvePreviewOutput{
		Body: *preview,
	}, nil
}

func (h *RemediationHandler) runLogAutoResolve(ctx context.Context, input *dto.RunLogAutoResolveInput) (*dto.RunLogAutoResolveOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	job, appErr := h.remediationService.RunAutoResolve(ctx, remediationservice.RunAutoResolveInput{
		OrganizationID: input.OrganizationID,
		LogID:          input.LogID,
		UserID:         session.UserID,
	})
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	out := &dto.RunLogAutoResolveOutput{}
	out.Body.JobID = job.ID
	return out, nil
}

func (h *RemediationHandler) ensureAdminOrOwner(ctx context.Context, userID string, organizationID string) apperr.Error {
	return h.authService.EnsureOrganizationRole(
		ctx,
		userID,
		organizationID,
		models.OrganizationMemberRoleOwner,
		models.OrganizationMemberRoleAdmin,
	)
}
