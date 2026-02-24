package errs

import (
	"time"

	"github.com/orvo-sh/orvo/pkg/apperr"
)

var (
	ErrArchiveNotConfigured = apperr.New(503, "archive_not_configured")
	ErrArchiveObjectMissing = apperr.New(404, "archive_object_not_found")
)

type RestoreRequiredError struct {
	Signal          string
	OrganizationID  string
	StartDay        time.Time
	EndDay          time.Time
	MissingDays     []time.Time
	RestorableDays  []time.Time
	QueuedRestoreID string
	QueuedState     string
}

func (e *RestoreRequiredError) Code() string {
	return "restore_required"
}

func (e *RestoreRequiredError) Status() int {
	return 409
}

func (e *RestoreRequiredError) Error() string {
	return "restore_required"
}
