package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrGithubNotConfigured       = apperr.New(503, "github_not_configured")
	ErrGithubInvalidState        = apperr.New(400, "github_invalid_state")
	ErrGithubStateExpired        = apperr.New(400, "github_state_expired")
	ErrGithubStateAlreadyUsed    = apperr.New(409, "github_state_already_used")
	ErrGithubInstallConflict     = apperr.New(409, "github_installation_already_linked")
	ErrGithubRepositoryNotFound  = apperr.New(404, "github_repository_not_found")
	ErrGithubRepositoryDisabled  = apperr.New(409, "github_repository_disabled")
	ErrGithubInvalidWebhook      = apperr.New(401, "github_invalid_webhook_signature")
	ErrGithubInstallationMissing = apperr.New(404, "github_installation_not_found")
)
