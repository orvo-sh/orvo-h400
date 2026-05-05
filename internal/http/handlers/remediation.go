package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/errs"
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
		OperationID: "list-auto-resolve-thresholds",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/remediation/auto-resolve-thresholds",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listAutoResolveThresholds)

	huma.Register(api, huma.Operation{
		OperationID: "upsert-auto-resolve-threshold",
		Method:      http.MethodPut,
		Path:        "/organizations/{organization_id}/remediation/auto-resolve-thresholds/{service_name}",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.upsertAutoResolveThreshold)

	huma.Register(api, huma.Operation{
		OperationID: "delete-auto-resolve-threshold",
		Method:      http.MethodDelete,
		Path:        "/organizations/{organization_id}/remediation/auto-resolve-thresholds/{service_name}",
		Tags:        []string{"remediation"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.deleteAutoResolveThreshold)

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

func (h *RemediationHandler) listAutoResolveThresholds(ctx context.Context, input *dto.ListAutoResolveThresholdsInput) (*dto.ListAutoResolveThresholdsOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	thresholds, appErr := h.remediationService.ListAutoResolveThresholds(ctx, input.OrganizationID)
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	out := &dto.ListAutoResolveThresholdsOutput{}
	for _, threshold := range thresholds {
		out.Body.Thresholds = append(out.Body.Thresholds, thresholdToDTO(threshold))
	}
	return out, nil
}

func (h *RemediationHandler) upsertAutoResolveThreshold(ctx context.Context, input *dto.UpsertAutoResolveThresholdInput) (*dto.UpsertAutoResolveThresholdOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	lookbackWindow, err := time.ParseDuration(strings.TrimSpace(input.Body.LookbackWindow))
	if err != nil {
		return nil, helpers.ToHTTPError(errs.ErrBadRequest)
	}
	cooldown, err := time.ParseDuration(strings.TrimSpace(input.Body.Cooldown))
	if err != nil {
		return nil, helpers.ToHTTPError(errs.ErrBadRequest)
	}

	threshold, appErr := h.remediationService.UpsertAutoResolveThreshold(ctx, remediationservice.UpsertAutoResolveThresholdInput{
		OrganizationID: input.OrganizationID,
		ServiceName:    input.ServiceName,
		UserID:         session.UserID,
		ThresholdValue: input.Body.ThresholdValue,
		LookbackWindow: lookbackWindow,
		Cooldown:       cooldown,
		Quorum:         input.Body.Quorum,
		Enabled:        input.Body.Enabled,
	})
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.UpsertAutoResolveThresholdOutput{
		Body: thresholdToDTO(*threshold),
	}, nil
}

func (h *RemediationHandler) deleteAutoResolveThreshold(ctx context.Context, input *dto.DeleteAutoResolveThresholdInput) (*dto.Empty, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if appErr := h.ensureAdminOrOwner(ctx, session.UserID, input.OrganizationID); appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	if appErr := h.remediationService.DeleteAutoResolveThreshold(ctx, input.OrganizationID, strings.TrimSpace(input.ServiceName)); appErr != nil {
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

func thresholdToDTO(threshold models.AutoResolveThreshold) dto.AutoResolveThreshold {
	out := dto.AutoResolveThreshold{
		ID:             threshold.ID,
		ServiceName:    threshold.ServiceName,
		MetricName:     threshold.MetricName,
		ThresholdValue: threshold.ThresholdValue,
		LookbackWindow: formatDurationCompact(time.Duration(threshold.LookbackWindowSeconds) * time.Second),
		Cooldown:       formatDurationCompact(time.Duration(threshold.CooldownSeconds) * time.Second),
		Quorum:         threshold.Quorum,
		Enabled:        threshold.Enabled,
		CreatedAt:      threshold.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      threshold.UpdatedAt.Format(time.RFC3339),
	}
	if threshold.LastTriggeredAt != nil {
		out.LastTriggeredAt = threshold.LastTriggeredAt.Format(time.RFC3339)
	}
	return out
}

func formatDurationCompact(value time.Duration) string {
	if value <= 0 {
		return "0s"
	}
	seconds := int(value.Seconds())
	switch {
	case seconds%3600 == 0:
		return fmt.Sprintf("%dh", seconds/3600)
	case seconds%60 == 0:
		return fmt.Sprintf("%dm", seconds/60)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
