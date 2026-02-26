package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/workers"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type SandboxHandler struct {
	authService    authservice.Service
	sandboxManager *workers.SandboxManager
}

func NewSandboxHandler(authService authservice.Service, sandboxManager *workers.SandboxManager) *SandboxHandler {
	return &SandboxHandler{
		authService:    authService,
		sandboxManager: sandboxManager,
	}
}

func (h *SandboxHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "list-sandbox-jobs",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/sandbox/jobs",
		Tags:        []string{"sandbox"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listJobs)

	huma.Register(api, huma.Operation{
		OperationID: "create-sandbox-job",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/sandbox/jobs",
		Tags:        []string{"sandbox"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createJob)

	huma.Register(api, huma.Operation{
		OperationID: "get-sandbox-job",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/sandbox/jobs/{job_id}",
		Tags:        []string{"sandbox"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getJob)

	huma.Register(api, huma.Operation{
		OperationID: "get-sandbox-job-logs",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/sandbox/jobs/{job_id}/logs",
		Tags:        []string{"sandbox"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getJobLogs)

	huma.Register(api, huma.Operation{
		OperationID: "cancel-sandbox-job",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/sandbox/jobs/{job_id}/cancel",
		Tags:        []string{"sandbox"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.cancelJob)
}

func (h *SandboxHandler) listJobs(ctx context.Context, input *dto.ListSandboxJobsInput) (*dto.ListSandboxJobsOutput, error) {
	var states []string
	for _, raw := range strings.Split(input.State, ",") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			states = append(states, trimmed)
		}
	}

	jobs, appErr := h.sandboxManager.ListJobs(ctx, input.OrganizationID, states, input.Limit)
	if appErr != nil {
		return nil, appErr
	}

	output := &dto.ListSandboxJobsOutput{}
	output.Body.Jobs = jobs
	return output, nil
}

func (h *SandboxHandler) createJob(ctx context.Context, input *dto.CreateSandboxJobInput) (*dto.CreateSandboxJobOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	draftPR := true
	if input.Body.DraftPR != nil {
		draftPR = *input.Body.DraftPR
	}

	job, appErr := h.sandboxManager.CreateJob(ctx, workers.CreateSandboxJobInput{
		OrganizationID: input.OrganizationID,
		RepositoryID:   input.Body.RepositoryID,
		RequestedBy:    session.UserID,
		BaseBranch:     input.Body.BaseBranch,
		TaskTitle:      input.Body.TaskTitle,
		CommitMessage:  input.Body.CommitMessage,
		Commands:       input.Body.Commands,
		DraftPR:        draftPR,
	})
	if appErr != nil {
		return nil, appErr
	}

	output := &dto.CreateSandboxJobOutput{}
	output.Body.JobID = job.ID
	return output, nil
}

func (h *SandboxHandler) getJob(ctx context.Context, input *dto.GetSandboxJobInput) (*dto.GetSandboxJobOutput, error) {
	job, appErr := h.sandboxManager.GetJob(ctx, input.OrganizationID, input.JobID)
	if appErr != nil {
		return nil, appErr
	}

	return &dto.GetSandboxJobOutput{
		Body: *job,
	}, nil
}

func (h *SandboxHandler) getJobLogs(ctx context.Context, input *dto.GetSandboxJobLogsInput) (*dto.GetSandboxJobLogsOutput, error) {
	logs, appErr := h.sandboxManager.GetLogs(ctx, input.OrganizationID, input.JobID, input.Cursor, input.Limit)
	if appErr != nil {
		return nil, appErr
	}

	output := &dto.GetSandboxJobLogsOutput{}
	output.Body.Logs = logs.Logs
	output.Body.NextCursor = logs.NextCursor
	return output, nil
}

func (h *SandboxHandler) cancelJob(ctx context.Context, input *dto.CancelSandboxJobInput) (*dto.Empty, error) {
	if appErr := h.sandboxManager.CancelJob(ctx, input.OrganizationID, input.JobID); appErr != nil {
		return nil, appErr
	}
	return nil, nil
}
