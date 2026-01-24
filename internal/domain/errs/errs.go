package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrInternal             = apperr.New(500, "internal_error")
	ErrBadRequest           = apperr.New(400, "bad_request")
	ErrMissingAuthorization = apperr.New(401, "missing_authorization")
)
