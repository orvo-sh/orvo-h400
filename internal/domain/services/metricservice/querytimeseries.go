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

type metricSQLBuilder struct {
	args []any
}

func (b *metricSQLBuilder) add(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}

func (s *service) QueryTimeseries(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "QueryTimeseries",
		slog.String("organization_id", input.OrganizationID),
		slog.String("metric_name", input.MetricName),
		slog.String("aggregation", input.Aggregation),
	)

	if isDerivedMetricName(input.MetricName) {
		return s.queryDerivedTimeseries(ctx, input)
	}

	if input.Aggregation == "" {
		input.Aggregation = "avg"
	}
	if input.StartTime.IsZero() {
		input.StartTime = time.Now().Add(-1 * time.Hour)
	}
	if input.EndTime.IsZero() {
		input.EndTime = time.Now()
	}
	if input.EndTime.Before(input.StartTime) {
		input.StartTime, input.EndTime = input.EndTime, input.StartTime
	}

	step := normalizeStep(input.StartTime, input.EndTime, input.Step)
	bucketSeconds := stepSeconds(step)

	args := &metricSQLBuilder{}
	hotWhere := buildMetricWhere("m", input, args)
	restoredWhere := buildMetricWhere("r", input, args)

	valueExpr := "coalesce(value_double, value_int::double precision, 0)"
	aggExpr := aggregateExpr(input.Aggregation)
	if isPercentileAgg(input.Aggregation) {
		aggExpr = fmt.Sprintf("percentile_cont(%s) WITHIN GROUP (ORDER BY u.metric_value)", percentileQuantile(input.Aggregation))
	}

	labelSelect := ""
	labelGroupBy := ""
	labelScanKeys := make([]string, 0, len(input.GroupBy))
	for _, key := range input.GroupBy {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		labelSelect += fmt.Sprintf(", coalesce(u.attributes->>'%s', '') AS \"%s\"", key, key)
		labelGroupBy += fmt.Sprintf(", \"%s\"", key)
		labelScanKeys = append(labelScanKeys, key)
	}

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			m.id,
			m.organization_id,
			m.metric_name,
			m.service_name,
			m.time,
			m.attributes,
			%s AS metric_value
		FROM metrics_hot m
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.metric_name,
			r.service_name,
			r.time,
			r.attributes,
			%s AS metric_value
		FROM metrics_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM metrics_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
		%s AS bucket,
		%s AS value
		%s
	FROM unioned u
	GROUP BY bucket%s
	ORDER BY bucket ASC`,
		valueExpr,
		hotWhere,
		valueExpr,
		restoredWhere,
		bucketExpression("u.time", step),
		aggExpr,
		labelSelect,
		labelGroupBy,
	)

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "QueryTimeseries: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	type seriesKey string
	seriesMap := make(map[seriesKey]*models.Timeseries)

	for rows.Next() {
		var (
			bucket time.Time
			value  float64
		)
		groupVals := make([]string, len(labelScanKeys))
		scanArgs := []any{&bucket, &value}
		for i := range labelScanKeys {
			scanArgs = append(scanArgs, &groupVals[i])
		}

		if err := rows.Scan(scanArgs...); err != nil {
			s.logger.ErrorContext(ctx, "QueryTimeseries: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		labels := make(map[string]string, len(labelScanKeys))
		keyParts := make([]string, 0, len(labelScanKeys))
		for i, key := range labelScanKeys {
			labels[key] = groupVals[i]
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", key, groupVals[i]))
		}
		key := strings.Join(keyParts, ",")

		ts, ok := seriesMap[seriesKey(key)]
		if !ok {
			ts = &models.Timeseries{Labels: labels}
			seriesMap[seriesKey(key)] = ts
		}

		point := models.TimeseriesPoint{Time: bucket, Value: value}
		if input.Aggregation == "rate" {
			if bucketSeconds > 0 {
				point.Value = value / float64(bucketSeconds)
			} else {
				point.Value = 0
			}
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

	sort.Slice(series, func(i, j int) bool {
		return fmt.Sprint(series[i].Labels) < fmt.Sprint(series[j].Labels)
	})

	return &QueryTimeseriesOutput{Series: series}, nil
}

func buildMetricWhere(alias string, input QueryTimeseriesInput, args *metricSQLBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
		fmt.Sprintf("%smetric_name = %s", prefix, args.add(input.MetricName)),
		fmt.Sprintf("%stime >= %s", prefix, args.add(input.StartTime)),
		fmt.Sprintf("%stime <= %s", prefix, args.add(input.EndTime)),
	}

	if input.ServiceName != "" {
		clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(input.ServiceName)))
	}

	filterKeys := make([]string, 0, len(input.Filters))
	for key := range input.Filters {
		filterKeys = append(filterKeys, key)
	}
	sort.Strings(filterKeys)
	for _, key := range filterKeys {
		clauses = append(clauses, fmt.Sprintf("%sattributes->>'%s' = %s", prefix, key, args.add(input.Filters[key])))
	}

	return strings.Join(clauses, " AND ")
}

func normalizeStep(start, end time.Time, step string) string {
	if step != "" {
		return step
	}

	d := end.Sub(start)
	switch {
	case d <= 2*time.Hour:
		return "1m"
	case d <= 12*time.Hour:
		return "5m"
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

func bucketExpression(column string, step string) string {
	seconds := stepSeconds(step)
	if seconds <= 0 {
		seconds = 60
	}
	return fmt.Sprintf("to_timestamp(floor(extract(epoch FROM %s) / %d) * %d)", column, seconds, seconds)
}

func aggregateExpr(aggregation string) string {
	switch aggregation {
	case "sum":
		return "sum(u.metric_value)"
	case "min":
		return "min(u.metric_value)"
	case "max":
		return "max(u.metric_value)"
	case "avg":
		return "avg(u.metric_value)"
	case "count":
		return "count(*)::double precision"
	case "last":
		return "(array_agg(u.metric_value ORDER BY u.time DESC))[1]"
	case "rate":
		return "sum(u.metric_value)"
	default:
		return "avg(u.metric_value)"
	}
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

func stepSeconds(step string) int {
	switch step {
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
		return 60
	}
}
