package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrInvalidCredentials    = apperr.New(401, "invalid_credentials")
	ErrEmailAlreadyExists    = apperr.New(409, "email_already_exists")
	ErrNotOrganizationMember = apperr.New(403, "not_organization_member")
	ErrSessionNotFound       = apperr.New(401, "session_not_found")
	ErrSessionExpired        = apperr.New(401, "session_expired")
	ErrUserNotFound          = apperr.New(404, "user_not_found")
)
