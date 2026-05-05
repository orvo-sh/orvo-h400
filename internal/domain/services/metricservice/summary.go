package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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

	if isDerivedMetricName(input.MetricName) {
		return s.queryDerivedSummary(ctx, input)
	}

	if input.Aggregation == "" {
		input.Aggregation = "avg"
	}

	lookback := input.LookbackWindow
	if lookback == 0 {
		lookback = 5 * time.Minute
	}
	since := time.Now().Add(-lookback)

	aggExpr := summaryAggExpr(input.Aggregation)

	args := &metricSQLBuilder{}
	hotWhere := buildSummaryWhere("m", input, since, args)
	restoredWhere := buildSummaryWhere("r", input, since, args)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			m.id,
			m.organization_id,
			m.metric_name,
			m.service_name,
			m.time,
			m.resource_attributes,
			m.attributes,
			coalesce(m.value_double, m.value_int::double precision, 0) AS metric_value
		FROM metrics_hot m
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.metric_name,
			r.service_name,
			r.time,
			r.resource_attributes,
			r.attributes,
			coalesce(r.value_double, r.value_int::double precision, 0) AS metric_value
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
		%s AS value,
		max(time) AS latest
	FROM unioned`, hotWhere, restoredWhere, aggExpr)

	row := s.pg.Pool().QueryRow(ctx, query, args.args...)

	var result GetMetricSummaryOutput
	if err := row.Scan(&result.Value, &result.Timestamp); err != nil {
		s.logger.ErrorContext(ctx, "GetMetricSummary: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &result, nil
}

func buildSummaryWhere(alias string, input GetMetricSummaryInput, since time.Time, args *metricSQLBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
		fmt.Sprintf("%smetric_name = %s", prefix, args.add(input.MetricName)),
		fmt.Sprintf("%stime >= %s", prefix, args.add(since)),
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
		clauses = append(clauses, fmt.Sprintf("%s = %s", jsonAttributeExpr(prefix, key), args.add(input.Filters[key])))
	}

	return strings.Join(clauses, " AND ")
}

func summaryAggExpr(aggregation string) string {
	switch aggregation {
	case "sum":
		return "sum(metric_value)"
	case "min":
		return "min(metric_value)"
	case "max":
		return "max(metric_value)"
	case "avg":
		return "avg(metric_value)"
	case "count":
		return "count(*)::double precision"
	case "last":
		return "(array_agg(metric_value ORDER BY time DESC))[1]"
	case "p50", "p90", "p95", "p99":
		return fmt.Sprintf("percentile_cont(%s) WITHIN GROUP (ORDER BY metric_value)", percentileQuantile(aggregation))
	default:
		return "avg(metric_value)"
	}
}
