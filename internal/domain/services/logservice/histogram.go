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
		return "1m"
	case d <= 2*time.Hour:
		return "5m"
	case d <= 12*time.Hour:
		return "15m"
	case d <= 48*time.Hour:
		return "1h"
	case d <= 7*24*time.Hour:
		return "6h"
	case d <= 30*24*time.Hour:
		return "1d"
	default:
		return "1d"
	}
}

func parseIntervalSeconds(interval string) int {
	switch interval {
	case "1m":
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	case "30m":
		return 1800
	case "1h":
		return 3600
	case "6h":
		return 21600
	case "12h":
		return 43200
	case "1d":
		return 86400
	default:
		return 3600
	}
}

func (s *service) GetHistogram(ctx context.Context, input GetHistogramInput) (*GetHistogramOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetHistogram: getting histogram", slog.Any("input", input))

	interval := input.Interval
	if interval == "" {
		interval = intervalFromRange(input.StartTime, input.EndTime)
	}
	stepSeconds := parseIntervalSeconds(interval)

	args := &sqlArgBuilder{}
	hotWhere := s.buildHistogramWhere(ctx, "l", input, args)
	restoredWhere := s.buildHistogramWhere(ctx, "r", input, args)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			l.id,
			l.organization_id,
			l.timestamp,
			l.severity_number
		FROM logs_hot l
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.timestamp,
			r.severity_number
		FROM logs_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM logs_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
		to_timestamp(floor(extract(epoch FROM unioned.timestamp) / %d) * %d) AS bucket,
		count(*) AS total,
		count(*) FILTER (WHERE severity_number >= 5 AND severity_number <= 8) AS debug,
		count(*) FILTER (WHERE severity_number >= 9 AND severity_number <= 12) AS info,
		count(*) FILTER (WHERE severity_number >= 13 AND severity_number <= 16) AS warn,
		count(*) FILTER (WHERE severity_number >= 17 AND severity_number <= 20) AS error,
		count(*) FILTER (WHERE severity_number >= 21 AND severity_number <= 24) AS fatal
	FROM unioned
	GROUP BY bucket
	ORDER BY bucket ASC`, hotWhere, restoredWhere, stepSeconds, stepSeconds)

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
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

func (s *service) buildHistogramWhere(ctx context.Context, alias string, input GetHistogramInput, args *sqlArgBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
	}

	if !input.StartTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%stimestamp >= %s", prefix, args.add(input.StartTime)))
	}
	if !input.EndTime.IsZero() {
		clauses = append(clauses, fmt.Sprintf("%stimestamp <= %s", prefix, args.add(input.EndTime)))
	}
	if input.SearchQuery != "" {
		clauses = append(clauses, fmt.Sprintf("%sbody ILIKE %s", prefix, args.add("%"+input.SearchQuery+"%")))
	}

	for _, f := range input.Filters {
		col, ok := resolveField(alias, f.Field)
		if !ok {
			s.logger.WarnContext(ctx, "GetHistogram: unknown filter field, skipping", slog.String("field", f.Field))
			continue
		}
		clause, err := buildFilterClause(col, f, args)
		if err != nil {
			s.logger.WarnContext(ctx, "GetHistogram: invalid filter, skipping",
				slog.String("field", f.Field),
				slog.String("operator", string(f.Operator)),
				slog.Any("error", err),
			)
			continue
		}
		clauses = append(clauses, clause)
	}

	return strings.Join(clauses, " AND ")
}
