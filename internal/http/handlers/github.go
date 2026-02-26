package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/githubservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type GithubHandler struct {
	authService   authservice.Service
	githubService githubservice.Service
}

func NewGithubHandler(authService authservice.Service, githubService githubservice.Service) *GithubHandler {
	return &GithubHandler{
		authService:   authService,
		githubService: githubService,
	}
}

func (h *GithubHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "create-github-install-url",
		Method:      http.MethodPost,
		Path:        "/organizations/{organization_id}/github/install-url",
		Tags:        []string{"github"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.createInstallURL)

	huma.Register(api, huma.Operation{
		OperationID: "list-github-installations",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/github/installations",
		Tags:        []string{"github"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listInstallations)

	huma.Register(api, huma.Operation{
		OperationID: "list-github-repositories",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/github/repositories",
		Tags:        []string{"github"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.listRepositories)

	huma.Register(api, huma.Operation{
		OperationID: "set-github-repository-enabled",
		Method:      http.MethodPatch,
		Path:        "/organizations/{organization_id}/github/repositories/{repository_id}",
		Tags:        []string{"github"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.setRepositoryEnabled)
}

func (h *GithubHandler) RegisterRawRoutes(router chi.Router) {
	router.Get("/github/setup/callback", h.setupCallbackHTTP)
	router.Post("/github/webhook", h.webhookHTTP)
}

func (h *GithubHandler) createInstallURL(ctx context.Context, input *dto.CreateGithubInstallURLInput) (*dto.CreateGithubInstallURLOutput, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if err := h.authService.EnsureOrganizationRole(ctx, session.UserID, input.OrganizationID, models.OrganizationMemberRoleOwner, models.OrganizationMemberRoleAdmin); err != nil {
		return nil, err
	}

	output, appErr := h.githubService.CreateInstallURL(ctx, githubservice.CreateInstallURLInput{
		OrganizationID: input.OrganizationID,
		UserID:         session.UserID,
	})
	if appErr != nil {
		return nil, appErr
	}

	response := &dto.CreateGithubInstallURLOutput{}
	response.Body.URL = output.URL
	return response, nil
}

func (h *GithubHandler) listInstallations(ctx context.Context, input *dto.ListGithubInstallationsInput) (*dto.ListGithubInstallationsOutput, error) {
	items, appErr := h.githubService.ListInstallations(ctx, input.OrganizationID)
	if appErr != nil {
		return nil, appErr
	}

	response := &dto.ListGithubInstallationsOutput{}
	response.Body.Installations = items
	return response, nil
}

func (h *GithubHandler) listRepositories(ctx context.Context, input *dto.ListGithubRepositoriesInput) (*dto.ListGithubRepositoriesOutput, error) {
	items, appErr := h.githubService.ListRepositories(ctx, input.OrganizationID)
	if appErr != nil {
		return nil, appErr
	}

	response := &dto.ListGithubRepositoriesOutput{}
	response.Body.Repositories = items
	return response, nil
}

func (h *GithubHandler) setRepositoryEnabled(ctx context.Context, input *dto.SetGithubRepositoryEnabledInput) (*dto.Empty, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if err := h.authService.EnsureOrganizationRole(ctx, session.UserID, input.OrganizationID, models.OrganizationMemberRoleOwner, models.OrganizationMemberRoleAdmin); err != nil {
		return nil, err
	}

	if appErr := h.githubService.SetRepositoryEnabled(ctx, githubservice.SetRepositoryEnabledInput{
		OrganizationID: input.OrganizationID,
		RepositoryID:   input.RepositoryID,
		Enabled:        input.Body.Enabled,
	}); appErr != nil {
		return nil, appErr
	}

	return nil, nil
}

func (h *GithubHandler) setupCallbackHTTP(w http.ResponseWriter, r *http.Request) {
	installationID, _ := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	setupAction := r.URL.Query().Get("setup_action")
	state := r.URL.Query().Get("state")

	output, appErr := h.githubService.HandleSetupCallback(r.Context(), githubservice.HandleSetupCallbackInput{
		InstallationID: installationID,
		SetupAction:    setupAction,
		State:          state,
	})
	if appErr != nil {
		redirectURL := "/settings?github=error&code=" + appErr.Code()
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	http.Redirect(w, r, output.RedirectURL, http.StatusFound)
}

func (h *GithubHandler) webhookHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "invalid_payload"})
		return
	}

	appErr := h.githubService.HandleWebhook(r.Context(), githubservice.HandleWebhookInput{
		Event:           r.Header.Get("X-GitHub-Event"),
		SignatureHeader: r.Header.Get("X-Hub-Signature-256"),
		Payload:         payload,
	})
	if appErr != nil {
		w.WriteHeader(appErr.Status())
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": appErr.Code()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
