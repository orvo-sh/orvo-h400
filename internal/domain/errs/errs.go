package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrInternal             = apperr.New(500, "internal_error")
	ErrBadRequest           = apperr.New(400, "bad_request")
	ErrMissingAuthorization = apperr.New(401, "missing_authorization")
	ErrNotFound             = apperr.New(404, "not_found")
	ErrApiKeyNotFound       = apperr.New(404, "api_key_not_found")
)
