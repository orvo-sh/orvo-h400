package helpers

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func ErrResp(ctx huma.Context, api huma.API, err apperr.Error) {
	huma.WriteErr(api, ctx, err.Status(), err.Code())
}

// GetCookie reads a cookie value from the huma context.
// The huma.Context embeds an http.Request accessible via the Cookie header.
func GetCookie(ctx huma.Context, name string) string {
	header := ctx.Header("Cookie")
	if header == "" {
		return ""
	}
	// Parse the Cookie header manually
	req := &http.Request{Header: http.Header{"Cookie": {header}}}
	cookie, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func ToHTTPError(err apperr.Error) error {
	if err == nil {
		return nil
	}

	if restoreErr, ok := err.(*errs.RestoreRequiredError); ok {
		return ToRestoreRequiredError(restoreErr)
	}

	return huma.NewError(err.Status(), err.Code())
}

func ToRestoreRequiredError(err *errs.RestoreRequiredError) error {
	details := []error{
		&huma.ErrorDetail{Location: "restore.signal", Message: "signal", Value: err.Signal},
		&huma.ErrorDetail{Location: "restore.start_day", Message: "start day", Value: err.StartDay.Format("2006-01-02")},
		&huma.ErrorDetail{Location: "restore.end_day", Message: "end day", Value: err.EndDay.Format("2006-01-02")},
	}

	if len(err.MissingDays) > 0 {
		details = append(details, &huma.ErrorDetail{
			Location: "restore.missing_days",
			Message:  "missing days",
			Value:    formatRestoreDays(err.MissingDays),
		})
	}
	if len(err.RestorableDays) > 0 {
		details = append(details, &huma.ErrorDetail{
			Location: "restore.restorable_days",
			Message:  "restorable days",
			Value:    formatRestoreDays(err.RestorableDays),
		})
	}
	if err.QueuedRestoreID != "" {
		details = append(details, &huma.ErrorDetail{
			Location: "restore.job_id",
			Message:  "queued restore job id",
			Value:    err.QueuedRestoreID,
		})
	}
	if err.QueuedState != "" {
		details = append(details, &huma.ErrorDetail{
			Location: "restore.job_state",
			Message:  "queued restore job state",
			Value:    err.QueuedState,
		})
	}

	return huma.NewError(http.StatusConflict, err.Code(), details...)
}

func formatRestoreDays(days []time.Time) []string {
	formatted := make([]string, 0, len(days))
	for _, day := range days {
		formatted = append(formatted, day.UTC().Format("2006-01-02"))
	}
	return formatted
}
