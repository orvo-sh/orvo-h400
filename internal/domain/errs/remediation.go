package errs

import "github.com/orvo-sh/orvo/pkg/apperr"

var (
	ErrAutoResolveNotError         = apperr.New(409, "auto_resolve_not_error")
	ErrAutoResolveOpencodeMissing  = apperr.New(503, "auto_resolve_opencode_not_configured")
	ErrAutoResolveContextTooLarge  = apperr.New(413, "auto_resolve_context_too_large")
	ErrRemediationMappingNotFound  = apperr.New(409, "auto_resolve_service_mapping_missing")
	ErrRemediationMappingNotUnique = apperr.New(409, "service_remediation_mapping_conflict")
)

type AutoResolveMappingMissingError struct {
	ServiceName string
}

func (e *AutoResolveMappingMissingError) Code() string {
	return "auto_resolve_service_mapping_missing"
}

func (e *AutoResolveMappingMissingError) Status() int {
	return 409
}

func (e *AutoResolveMappingMissingError) Error() string {
	return "auto_resolve_service_mapping_missing"
}
