package logservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) checkRestoreRequired(ctx context.Context, input QueryLogsInput) apperr.Error {
	if input.StartTime.IsZero() || input.EndTime.IsZero() || s.hotRetention <= 0 {
		return nil
	}

	startDay, endDay := normalizeDays(input.StartTime, input.EndTime)
	cutoffDay := truncateDay(time.Now().UTC().Add(-s.hotRetention))

	olderDays := make([]time.Time, 0)
	for _, day := range listDays(startDay, endDay) {
		if day.Before(cutoffDay) {
			olderDays = append(olderDays, day)
		}
	}
	if len(olderDays) == 0 {
		return nil
	}

	covered, err := s.fetchCoveredDays(ctx, input.OrganizationID, "logs", olderDays)
	if err != nil {
		return errs.ErrInternal
	}

	missing := make([]time.Time, 0, len(olderDays))
	for _, day := range olderDays {
		if _, ok := covered[day.Format("2006-01-02")]; !ok {
			missing = append(missing, day)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	restorable, err := s.fetchArchiveDays(ctx, input.OrganizationID, "logs", missing)
	if err != nil {
		return errs.ErrInternal
	}
	if len(restorable) == 0 {
		return nil
	}

	missingRestorable := intersectDays(missing, restorable)
	if len(missingRestorable) == 0 {
		return nil
	}

	jobID, jobState, err := s.fetchPendingRestoreJob(ctx, input.OrganizationID, "logs", startDay, endDay)
	if err != nil {
		return errs.ErrInternal
	}

	return &errs.RestoreRequiredError{
		Signal:          "logs",
		OrganizationID:  input.OrganizationID,
		StartDay:        startDay,
		EndDay:          endDay,
		MissingDays:     missingRestorable,
		RestorableDays:  restorable,
		QueuedRestoreID: jobID,
		QueuedState:     jobState,
	}
}

func (s *service) fetchCoveredDays(ctx context.Context, organizationID string, signal string, days []time.Time) (map[string]struct{}, error) {
	args := make([]any, 0, len(days)+2)
	args = append(args, organizationID, signal)

	inClause := dayInClause(days, &args, 3)
	query := fmt.Sprintf(`
		SELECT day
		FROM restored_coverage
		WHERE organization_id = $1
		  AND signal = $2
		  AND expires_at > NOW()
		  AND day IN (%s)
	`, inClause)

	rows, err := s.pg.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	covered := make(map[string]struct{}, len(days))
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		covered[truncateDay(day).Format("2006-01-02")] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return covered, nil
}

func (s *service) fetchArchiveDays(ctx context.Context, organizationID string, signal string, days []time.Time) ([]time.Time, error) {
	args := make([]any, 0, len(days)+2)
	args = append(args, organizationID, signal)

	inClause := dayInClause(days, &args, 3)
	query := fmt.Sprintf(`
		SELECT DISTINCT day
		FROM archive_objects
		WHERE organization_id = $1
		  AND signal = $2
		  AND deleted_at IS NULL
		  AND day IN (%s)
		ORDER BY day ASC
	`, inClause)

	rows, err := s.pg.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]time.Time, 0)
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		result = append(result, truncateDay(day))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) fetchPendingRestoreJob(
	ctx context.Context,
	organizationID string,
	signal string,
	startDay time.Time,
	endDay time.Time,
) (string, string, error) {
	var jobID string
	var jobState string
	err := s.pg.Pool().QueryRow(ctx, `
		SELECT id, state
		FROM restore_jobs
		WHERE organization_id = $1
		  AND signal = $2
		  AND state IN ('queued', 'running')
		  AND start_day <= $3
		  AND end_day >= $4
		ORDER BY created_at DESC
		LIMIT 1
	`, organizationID, signal, endDay, startDay).Scan(&jobID, &jobState)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}

	return jobID, jobState, nil
}

func dayInClause(days []time.Time, args *[]any, startIdx int) string {
	placeholders := make([]string, 0, len(days))
	for _, day := range days {
		*args = append(*args, truncateDay(day))
		placeholders = append(placeholders, fmt.Sprintf("$%d::date", startIdx+len(placeholders)))
	}
	if len(placeholders) == 0 {
		return "NULL"
	}
	return strings.Join(placeholders, ", ")
}

func listDays(startDay time.Time, endDay time.Time) []time.Time {
	days := make([]time.Time, 0, int(endDay.Sub(startDay).Hours()/24)+1)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		days = append(days, day)
	}
	return days
}

func normalizeDays(start time.Time, end time.Time) (time.Time, time.Time) {
	startDay := truncateDay(start)
	endDay := truncateDay(end)
	if endDay.Before(startDay) {
		return endDay, startDay
	}
	return startDay, endDay
}

func truncateDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func intersectDays(a []time.Time, b []time.Time) []time.Time {
	set := make(map[string]struct{}, len(b))
	for _, day := range b {
		set[truncateDay(day).Format("2006-01-02")] = struct{}{}
	}

	out := make([]time.Time, 0, len(a))
	for _, day := range a {
		dayKey := truncateDay(day).Format("2006-01-02")
		if _, ok := set[dayKey]; ok {
			out = append(out, truncateDay(day))
		}
	}
	return out
}
