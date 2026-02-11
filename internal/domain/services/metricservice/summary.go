package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetMetricSummary(ctx context.Context, input GetMetricSummaryInput) (*GetMetricSummaryOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetMetricSummary",
		slog.String("organization_id", input.OrganizationID),
		slog.String("metric_name", input.MetricName),
		slog.String("aggregation", input.Aggregation),
	)

	if input.Aggregation == "" {
		input.Aggregation = "avg"
	}

	lookback := input.LookbackWindow
	if lookback == 0 {
		lookback = 5 * time.Minute
	}

	since := time.Now().Add(-lookback)

	var aggExpr string
	switch input.Aggregation {
	case "sum":
		return s.summaryFromRollup(ctx, input, since, "sum(sum_value)")
	case "min":
		return s.summaryFromRollup(ctx, input, since, "min(min_value)")
	case "max":
		return s.summaryFromRollup(ctx, input, since, "max(max_value)")
	case "avg":
		return s.summaryFromRollup(ctx, input, since, "avgMerge(avg_value)")
	case "count":
		return s.summaryFromRollup(ctx, input, since, "sum(point_count)")
	case "last":
		return s.summaryFromRollup(ctx, input, since, "argMaxMerge(last_value)")
	case "p50", "p90", "p95", "p99":
		quantile := percentileQuantile(input.Aggregation)
		aggExpr = fmt.Sprintf("quantile(%s)(coalesce(value_double, CAST(value_int AS Float64), 0))", quantile)
		return s.summaryFromRaw(ctx, input, since, aggExpr)
	default:
		return s.summaryFromRollup(ctx, input, since, "avgMerge(avg_value)")
	}
}

func (s *service) summaryFromRollup(ctx context.Context, input GetMetricSummaryInput, since time.Time, aggExpr string) (*GetMetricSummaryOutput, apperr.Error) {
	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	clauses = append(clauses, "metric_name = ?")
	args = append(args, input.MetricName)

	clauses = append(clauses, "time_bucket >= ?")
	args = append(args, since)

	if input.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, input.ServiceName)
	}

	for k, v := range input.Filters {
		clauses = append(clauses, fmt.Sprintf("attributes['%s'] = ?", k))
		args = append(args, v)
	}

	where := strings.Join(clauses, " AND ")

	query := fmt.Sprintf(`SELECT
		%s AS value,
		max(time_bucket) AS latest
	FROM metrics_1m
	WHERE %s`, aggExpr, where)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetMetricSummary(rollup): query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	result := &GetMetricSummaryOutput{}
	if rows.Next() {
		if err := rows.Scan(&result.Value, &result.Timestamp); err != nil {
			s.logger.ErrorContext(ctx, "GetMetricSummary(rollup): scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetMetricSummary(rollup): rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return result, nil
}

func (s *service) summaryFromRaw(ctx context.Context, input GetMetricSummaryInput, since time.Time, aggExpr string) (*GetMetricSummaryOutput, apperr.Error) {
	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	clauses = append(clauses, "metric_name = ?")
	args = append(args, input.MetricName)

	clauses = append(clauses, "time >= ?")
	args = append(args, since)

	if input.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, input.ServiceName)
	}

	for k, v := range input.Filters {
		clauses = append(clauses, fmt.Sprintf("attributes['%s'] = ?", k))
		args = append(args, v)
	}

	where := strings.Join(clauses, " AND ")

	query := fmt.Sprintf(`SELECT
		%s AS value,
		max(time) AS latest
	FROM metrics
	WHERE %s`, aggExpr, where)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetMetricSummary(raw): query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	result := &GetMetricSummaryOutput{}
	if rows.Next() {
		if err := rows.Scan(&result.Value, &result.Timestamp); err != nil {
			s.logger.ErrorContext(ctx, "GetMetricSummary(raw): scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetMetricSummary(raw): rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return result, nil
}
