package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrSandboxNotConfigured = apperr.New(503, "sandbox_not_configured")
	ErrSandboxJobNotFound   = apperr.New(404, "sandbox_job_not_found")
)
