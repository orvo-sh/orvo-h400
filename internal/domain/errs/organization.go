package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrOrgNotFound = apperr.New(404, "organization_not_found")
)
