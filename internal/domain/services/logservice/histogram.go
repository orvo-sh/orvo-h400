package logservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

// intervalFromRange calculates a reasonable interval based on the time range.
func intervalFromRange(start, end time.Time) string {
	d := end.Sub(start)
	switch {
	case d <= 30*time.Minute:
		return "1 MINUTE"
	case d <= 2*time.Hour:
		return "5 MINUTE"
	case d <= 12*time.Hour:
		return "15 MINUTE"
	case d <= 48*time.Hour:
		return "1 HOUR"
	case d <= 7*24*time.Hour:
		return "6 HOUR"
	case d <= 30*24*time.Hour:
		return "1 DAY"
	default:
		return "1 DAY"
	}
}

// parseInterval converts user-provided shorthand intervals to ClickHouse interval strings.
func parseInterval(s string) string {
	switch s {
	case "1m":
		return "1 MINUTE"
	case "5m":
		return "5 MINUTE"
	case "15m":
		return "15 MINUTE"
	case "30m":
		return "30 MINUTE"
	case "1h":
		return "1 HOUR"
	case "6h":
		return "6 HOUR"
	case "12h":
		return "12 HOUR"
	case "1d":
		return "1 DAY"
	default:
		return s
	}
}

func (s *service) GetHistogram(ctx context.Context, input GetHistogramInput) (*GetHistogramOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetHistogram: getting histogram", slog.Any("input", input))

	interval := input.Interval
	if interval == "" {
		interval = intervalFromRange(input.StartTime, input.EndTime)
	} else {
		interval = parseInterval(interval)
	}

	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	if !input.StartTime.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, input.StartTime)
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, input.EndTime)
	}

	if input.SearchQuery != "" {
		clauses = append(clauses, "body ILIKE ?")
		args = append(args, "%"+input.SearchQuery+"%")
	}

	for _, f := range input.Filters {
		col, ok := resolveField(f.Field)
		if !ok {
			continue
		}
		clause, filterArgs, err := buildFilterClause(col, f)
		if err != nil {
			continue
		}
		clauses = append(clauses, clause)
		args = append(args, filterArgs...)
	}

	where := strings.Join(clauses, " AND ")

	// Severity ranges per OTLP spec:
	// TRACE=1-4, DEBUG=5-8, INFO=9-12, WARN=13-16, ERROR=17-20, FATAL=21-24
	query := fmt.Sprintf(`SELECT
		toStartOfInterval(timestamp, INTERVAL %s) AS bucket,
		count() AS total,
		countIf(severity_number >= 5 AND severity_number <= 8) AS debug,
		countIf(severity_number >= 9 AND severity_number <= 12) AS info,
		countIf(severity_number >= 13 AND severity_number <= 16) AS warn,
		countIf(severity_number >= 17 AND severity_number <= 20) AS error,
		countIf(severity_number >= 21 AND severity_number <= 24) AS fatal
	FROM logs
	WHERE %s
	GROUP BY bucket
	ORDER BY bucket ASC`, interval, where)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetHistogram: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var buckets []HistogramBucket
	for rows.Next() {
		var b HistogramBucket
		if err := rows.Scan(
			&b.Time,
			&b.Count,
			&b.Debug,
			&b.Info,
			&b.Warn,
			&b.Error,
			&b.Fatal,
		); err != nil {
			s.logger.ErrorContext(ctx, "GetHistogram: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetHistogram: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &GetHistogramOutput{Buckets: buckets}, nil
}
