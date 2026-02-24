package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/workers"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/helpers"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type ArchiveHandler struct {
	authService    authservice.Service
	restoreManager *workers.RestoreManager
}

func NewArchiveHandler(authService authservice.Service, restoreManager *workers.RestoreManager) *ArchiveHandler {
	return &ArchiveHandler{
		authService:    authService,
		restoreManager: restoreManager,
	}
}

func (h *ArchiveHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "create-restore-job",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/archive/restores",
		Tags:        []string{"archive"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createRestoreJob)

	huma.Register(api, huma.Operation{
		OperationID: "get-restore-job",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/archive/restores/{restore_job_id}",
		Tags:        []string{"archive"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getRestoreJob)
}

func (h *ArchiveHandler) createRestoreJob(ctx context.Context, input *dto.CreateRestoreJobInput) (*dto.CreateRestoreJobOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)

	signal, ok := workers.ParseTelemetrySignal(input.Body.Signal)
	if !ok {
		return nil, huma.NewError(http.StatusBadRequest, "invalid_signal")
	}

	startDay, err := time.ParseInLocation("2006-01-02", input.Body.StartDay, time.UTC)
	if err != nil {
		return nil, huma.NewError(http.StatusBadRequest, "invalid_start_day")
	}

	endDay, err := time.ParseInLocation("2006-01-02", input.Body.EndDay, time.UTC)
	if err != nil {
		return nil, huma.NewError(http.StatusBadRequest, "invalid_end_day")
	}

	job, appErr := h.restoreManager.CreateJob(ctx, workers.CreateRestoreJobInput{
		OrganizationID: input.OrganizationID,
		Signal:         signal,
		StartDay:       startDay,
		EndDay:         endDay,
		RequestedBy:    session.UserID,
	})
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.CreateRestoreJobOutput{Body: *job}, nil
}

func (h *ArchiveHandler) getRestoreJob(ctx context.Context, input *dto.GetRestoreJobInput) (*dto.GetRestoreJobOutput, error) {
	job, appErr := h.restoreManager.GetJob(ctx, input.OrganizationID, input.RestoreJobID)
	if appErr != nil {
		return nil, helpers.ToHTTPError(appErr)
	}

	return &dto.GetRestoreJobOutput{Body: *job}, nil
}
