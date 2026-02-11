package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

// rollupTable selects the best rollup table based on the query time range and requested step.
// Returns the table name and the ClickHouse interval string to use for bucketing.
func rollupTable(start, end time.Time, step string) (table string, interval string) {
	d := end.Sub(start)

	// If an explicit step is requested, use the most appropriate rollup.
	if step != "" {
		switch step {
		case "1m":
			return "metrics_1m", "1 MINUTE"
		case "5m":
			return "metrics_1m", "5 MINUTE"
		case "15m":
			return "metrics_1m", "15 MINUTE"
		case "30m":
			return "metrics_1m", "30 MINUTE"
		case "1h":
			return "metrics_1h", "1 HOUR"
		case "6h":
			return "metrics_1h", "6 HOUR"
		case "12h":
			return "metrics_1h", "12 HOUR"
		case "1d":
			return "metrics_1d", "1 DAY"
		}
	}

	// Auto-select based on time range.
	switch {
	case d <= 2*time.Hour:
		return "metrics_1m", "1 MINUTE"
	case d <= 12*time.Hour:
		return "metrics_1m", "5 MINUTE"
	case d <= 48*time.Hour:
		return "metrics_1h", "1 HOUR"
	case d <= 7*24*time.Hour:
		return "metrics_1h", "6 HOUR"
	case d <= 30*24*time.Hour:
		return "metrics_1d", "1 DAY"
	default:
		return "metrics_1d", "1 DAY"
	}
}

// aggExpression returns the ClickHouse SELECT expression for a given aggregation function
// operating on rollup table columns.
func aggExpression(aggregation string) string {
	switch aggregation {
	case "sum":
		return "sum(sum_value)"
	case "min":
		return "min(min_value)"
	case "max":
		return "max(max_value)"
	case "avg":
		return "avgMerge(avg_value)"
	case "count":
		return "sum(point_count)"
	case "last":
		return "argMaxMerge(last_value)"
	case "rate":
		// rate = sum of values / number of seconds in the interval
		// We'll compute it in post-processing since the interval width varies.
		return "sum(sum_value)"
	default:
		return "avgMerge(avg_value)"
	}
}

func (s *service) QueryTimeseries(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryTimeseries",
		slog.String("organization_id", input.OrganizationID),
		slog.String("metric_name", input.MetricName),
		slog.String("aggregation", input.Aggregation),
	)

	if input.Aggregation == "" {
		input.Aggregation = "avg"
	}

	// For percentile queries, we must use the raw metrics table.
	if isPercentileAgg(input.Aggregation) {
		return s.queryPercentilesFromRaw(ctx, input)
	}

	table, interval := rollupTable(input.StartTime, input.EndTime, input.Step)

	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	clauses = append(clauses, "metric_name = ?")
	args = append(args, input.MetricName)

	clauses = append(clauses, "time_bucket >= ?")
	args = append(args, input.StartTime)

	clauses = append(clauses, "time_bucket <= ?")
	args = append(args, input.EndTime)

	if input.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, input.ServiceName)
	}

	for k, v := range input.Filters {
		clauses = append(clauses, fmt.Sprintf("attributes['%s'] = ?", k))
		args = append(args, v)
	}

	where := strings.Join(clauses, " AND ")

	// Build GROUP BY: always group by time bucket; optionally by attribute keys.
	groupByCols := []string{"bucket"}
	selectExtra := ""
	for _, g := range input.GroupBy {
		col := fmt.Sprintf("attributes['%s']", g)
		groupByCols = append(groupByCols, col)
		selectExtra += fmt.Sprintf(", %s AS `%s`", col, g)
	}

	aggExpr := aggExpression(input.Aggregation)
	groupBySQL := strings.Join(groupByCols, ", ")

	query := fmt.Sprintf(`SELECT
		toStartOfInterval(time_bucket, INTERVAL %s) AS bucket,
		%s AS value%s
	FROM %s
	WHERE %s
	GROUP BY %s
	ORDER BY bucket ASC`,
		interval, aggExpr, selectExtra, table, where, groupBySQL)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryTimeseries: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	// seriesMap groups points by their label set.
	type seriesKey = string
	seriesMap := make(map[seriesKey]*models.Timeseries)

	for rows.Next() {
		var (
			bucket time.Time
			value  float64
		)
		groupVals := make([]string, len(input.GroupBy))
		scanArgs := []any{&bucket, &value}
		for i := range input.GroupBy {
			scanArgs = append(scanArgs, &groupVals[i])
		}

		if err := rows.Scan(scanArgs...); err != nil {
			s.logger.ErrorContext(ctx, "QueryTimeseries: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		// Build label key.
		labels := make(map[string]string, len(input.GroupBy))
		keyParts := make([]string, 0, len(input.GroupBy))
		for i, g := range input.GroupBy {
			labels[g] = groupVals[i]
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", g, groupVals[i]))
		}
		key := strings.Join(keyParts, ",")

		ts, ok := seriesMap[key]
		if !ok {
			ts = &models.Timeseries{Labels: labels}
			seriesMap[key] = ts
		}

		point := models.TimeseriesPoint{Time: bucket, Value: value}
		if input.Aggregation == "rate" {
			// Convert sum-per-bucket to per-second rate.
			point.Value = ratePerSecond(value, interval)
		}
		ts.Points = append(ts.Points, point)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "QueryTimeseries: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	series := make([]models.Timeseries, 0, len(seriesMap))
	for _, ts := range seriesMap {
		series = append(series, *ts)
	}

	// Sort series deterministically by label key.
	sort.Slice(series, func(i, j int) bool {
		return fmt.Sprint(series[i].Labels) < fmt.Sprint(series[j].Labels)
	})

	return &QueryTimeseriesOutput{Series: series}, nil
}

// queryPercentilesFromRaw queries percentiles directly from the raw metrics table.
func (s *service) queryPercentilesFromRaw(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error) {
	_, interval := rollupTable(input.StartTime, input.EndTime, input.Step)
	quantile := percentileQuantile(input.Aggregation)

	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	clauses = append(clauses, "metric_name = ?")
	args = append(args, input.MetricName)

	clauses = append(clauses, "time >= ?")
	args = append(args, input.StartTime)

	clauses = append(clauses, "time <= ?")
	args = append(args, input.EndTime)

	if input.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, input.ServiceName)
	}

	for k, v := range input.Filters {
		clauses = append(clauses, fmt.Sprintf("attributes['%s'] = ?", k))
		args = append(args, v)
	}

	where := strings.Join(clauses, " AND ")

	groupByCols := []string{"bucket"}
	selectExtra := ""
	for _, g := range input.GroupBy {
		col := fmt.Sprintf("attributes['%s']", g)
		groupByCols = append(groupByCols, col)
		selectExtra += fmt.Sprintf(", %s AS `%s`", col, g)
	}

	groupBySQL := strings.Join(groupByCols, ", ")

	query := fmt.Sprintf(`SELECT
		toStartOfInterval(time, INTERVAL %s) AS bucket,
		quantile(%s)(coalesce(value_double, CAST(value_int AS Float64), 0)) AS value%s
	FROM metrics
	WHERE %s
	GROUP BY %s
	ORDER BY bucket ASC`,
		interval, quantile, selectExtra, where, groupBySQL)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryTimeseries(percentile): query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	type seriesKey = string
	seriesMap := make(map[seriesKey]*models.Timeseries)

	for rows.Next() {
		var (
			bucket time.Time
			value  float64
		)
		groupVals := make([]string, len(input.GroupBy))
		scanArgs := []any{&bucket, &value}
		for i := range input.GroupBy {
			scanArgs = append(scanArgs, &groupVals[i])
		}

		if err := rows.Scan(scanArgs...); err != nil {
			s.logger.ErrorContext(ctx, "QueryTimeseries(percentile): scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		labels := make(map[string]string, len(input.GroupBy))
		keyParts := make([]string, 0, len(input.GroupBy))
		for i, g := range input.GroupBy {
			labels[g] = groupVals[i]
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", g, groupVals[i]))
		}
		key := strings.Join(keyParts, ",")

		ts, ok := seriesMap[key]
		if !ok {
			ts = &models.Timeseries{Labels: labels}
			seriesMap[key] = ts
		}
		ts.Points = append(ts.Points, models.TimeseriesPoint{Time: bucket, Value: value})
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "QueryTimeseries(percentile): rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	series := make([]models.Timeseries, 0, len(seriesMap))
	for _, ts := range seriesMap {
		series = append(series, *ts)
	}

	sort.Slice(series, func(i, j int) bool {
		return fmt.Sprint(series[i].Labels) < fmt.Sprint(series[j].Labels)
	})

	return &QueryTimeseriesOutput{Series: series}, nil
}

func isPercentileAgg(agg string) bool {
	switch agg {
	case "p50", "p90", "p95", "p99":
		return true
	}
	return false
}

func percentileQuantile(agg string) string {
	switch agg {
	case "p50":
		return "0.5"
	case "p90":
		return "0.9"
	case "p95":
		return "0.95"
	case "p99":
		return "0.99"
	default:
		return "0.5"
	}
}

// ratePerSecond divides a sum-per-bucket value by the bucket width in seconds.
func ratePerSecond(sum float64, interval string) float64 {
	seconds := intervalSeconds(interval)
	if seconds == 0 {
		return 0
	}
	return sum / float64(seconds)
}

func intervalSeconds(interval string) int {
	switch interval {
	case "1 MINUTE":
		return 60
	case "5 MINUTE":
		return 300
	case "15 MINUTE":
		return 900
	case "30 MINUTE":
		return 1800
	case "1 HOUR":
		return 3600
	case "6 HOUR":
		return 21600
	case "12 HOUR":
		return 43200
	case "1 DAY":
		return 86400
	default:
		return 60
	}
}
