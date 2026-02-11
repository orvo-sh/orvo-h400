package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetREDMetrics(ctx context.Context, input GetREDMetricsInput) (*models.REDMetrics, apperr.Error) {
	s.logger.InfoContext(ctx, "GetREDMetrics",
		slog.String("organization_id", input.OrganizationID),
		slog.String("service_name", input.ServiceName),
	)

	table, interval := rollupTable(input.StartTime, input.EndTime, input.Step)

	result := &models.REDMetrics{}

	// 1. Request rate (from spans.request.count)
	requestRate, err := s.queryREDTimeseries(ctx, table, interval, input, "spans.request.count", "sum")
	if err != nil {
		return nil, err
	}
	// Convert to per-second rate.
	seconds := float64(intervalSeconds(interval))
	for i := range requestRate {
		if seconds > 0 {
			requestRate[i].Value = requestRate[i].Value / seconds
		}
	}
	result.RequestRate = requestRate

	// 2. Error rate (from spans.error.count)
	errorRate, appErr := s.queryREDTimeseries(ctx, table, interval, input, "spans.error.count", "sum")
	if appErr != nil {
		return nil, appErr
	}
	for i := range errorRate {
		if seconds > 0 {
			errorRate[i].Value = errorRate[i].Value / seconds
		}
	}
	result.ErrorRate = errorRate

	// 3. Duration percentiles (from spans.duration, raw table)
	p50, appErr := s.queryDurationPercentile(ctx, interval, input, "0.5")
	if appErr != nil {
		return nil, appErr
	}
	result.P50Latency = p50

	p90, appErr := s.queryDurationPercentile(ctx, interval, input, "0.9")
	if appErr != nil {
		return nil, appErr
	}
	result.P90Latency = p90

	p95, appErr := s.queryDurationPercentile(ctx, interval, input, "0.95")
	if appErr != nil {
		return nil, appErr
	}
	result.P95Latency = p95

	p99, appErr := s.queryDurationPercentile(ctx, interval, input, "0.99")
	if appErr != nil {
		return nil, appErr
	}
	result.P99Latency = p99

	return result, nil
}

// queryREDTimeseries queries a single RED metric from a rollup table.
func (s *service) queryREDTimeseries(
	ctx context.Context,
	table, interval string,
	input GetREDMetricsInput,
	metricName, agg string,
) ([]models.TimeseriesPoint, apperr.Error) {
	aggExpr := aggExpression(agg)

	query := fmt.Sprintf(`SELECT
		toStartOfInterval(time_bucket, INTERVAL %s) AS bucket,
		%s AS value
	FROM %s
	WHERE organization_id = ?
	  AND metric_name = ?
	  AND time_bucket >= ?
	  AND time_bucket <= ?
	  AND service_name = ?
	GROUP BY bucket
	ORDER BY bucket ASC`, interval, aggExpr, table)

	args := []any{input.OrganizationID, metricName, input.StartTime, input.EndTime, input.ServiceName}

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "queryREDTimeseries: query failed",
			slog.String("metric_name", metricName),
			slog.Any("error", err),
		)
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var points []models.TimeseriesPoint
	for rows.Next() {
		var p models.TimeseriesPoint
		if err := rows.Scan(&p.Time, &p.Value); err != nil {
			s.logger.ErrorContext(ctx, "queryREDTimeseries: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "queryREDTimeseries: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return points, nil
}

// queryDurationPercentile queries a percentile from the raw metrics table for spans.duration.
func (s *service) queryDurationPercentile(
	ctx context.Context,
	interval string,
	input GetREDMetricsInput,
	quantile string,
) ([]models.TimeseriesPoint, apperr.Error) {
	query := fmt.Sprintf(`SELECT
		toStartOfInterval(time, INTERVAL %s) AS bucket,
		quantile(%s)(coalesce(value_double, CAST(value_int AS Float64), 0)) AS value
	FROM metrics
	WHERE organization_id = ?
	  AND metric_name = 'spans.duration'
	  AND time >= ?
	  AND time <= ?
	  AND service_name = ?
	GROUP BY bucket
	ORDER BY bucket ASC`, interval, quantile)

	args := []any{input.OrganizationID, input.StartTime, input.EndTime, input.ServiceName}

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "queryDurationPercentile: query failed",
			slog.String("quantile", quantile),
			slog.Any("error", err),
		)
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var points []models.TimeseriesPoint
	for rows.Next() {
		var p models.TimeseriesPoint
		if err := rows.Scan(&p.Time, &p.Value); err != nil {
			s.logger.ErrorContext(ctx, "queryDurationPercentile: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "queryDurationPercentile: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return points, nil
}

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
	default:
		return "1 DAY"
	}
}
