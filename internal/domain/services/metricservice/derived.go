package metricservice

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

const (
	derivedMetricRequestsRate   = "derived.requests.rate"
	derivedMetricRequestsTotal  = "derived.requests.total"
	derivedMetricErrorsRate     = "derived.errors.rate"
	derivedMetricErrorsTotal    = "derived.errors.total"
	derivedMetricErrorsRatioPct = "derived.errors.ratio_pct"
	derivedMetricLatencyAvgMS   = "derived.latency.avg_ms"
	derivedMetricLatencyP50MS   = "derived.latency.p50_ms"
	derivedMetricLatencyP90MS   = "derived.latency.p90_ms"
	derivedMetricLatencyP95MS   = "derived.latency.p95_ms"
	derivedMetricLatencyP99MS   = "derived.latency.p99_ms"
)

var derivedMetricCatalog = []models.MetricMeta{
	{
		Name:        derivedMetricRequestsRate,
		Type:        "derived",
		Unit:        "req/s",
		Description: "Request rate derived from server spans",
	},
	{
		Name:        derivedMetricRequestsTotal,
		Type:        "derived",
		Unit:        "requests",
		Description: "Request count derived from server spans",
	},
	{
		Name:        derivedMetricErrorsRate,
		Type:        "derived",
		Unit:        "err/s",
		Description: "Error rate derived from server spans (status_code=2)",
	},
	{
		Name:        derivedMetricErrorsTotal,
		Type:        "derived",
		Unit:        "errors",
		Description: "Error count derived from server spans (status_code=2)",
	},
	{
		Name:        derivedMetricErrorsRatioPct,
		Type:        "derived",
		Unit:        "%",
		Description: "Error percentage derived from server spans",
	},
	{
		Name:        derivedMetricLatencyAvgMS,
		Type:        "derived",
		Unit:        "ms",
		Description: "Average latency in milliseconds derived from server spans",
	},
	{
		Name:        derivedMetricLatencyP50MS,
		Type:        "derived",
		Unit:        "ms",
		Description: "P50 latency in milliseconds derived from server spans",
	},
	{
		Name:        derivedMetricLatencyP90MS,
		Type:        "derived",
		Unit:        "ms",
		Description: "P90 latency in milliseconds derived from server spans",
	},
	{
		Name:        derivedMetricLatencyP95MS,
		Type:        "derived",
		Unit:        "ms",
		Description: "P95 latency in milliseconds derived from server spans",
	},
	{
		Name:        derivedMetricLatencyP99MS,
		Type:        "derived",
		Unit:        "ms",
		Description: "P99 latency in milliseconds derived from server spans",
	},
}

func isDerivedMetricName(metricName string) bool {
	switch strings.TrimSpace(strings.ToLower(metricName)) {
	case derivedMetricRequestsRate,
		derivedMetricRequestsTotal,
		derivedMetricErrorsRate,
		derivedMetricErrorsTotal,
		derivedMetricErrorsRatioPct,
		derivedMetricLatencyAvgMS,
		derivedMetricLatencyP50MS,
		derivedMetricLatencyP90MS,
		derivedMetricLatencyP95MS,
		derivedMetricLatencyP99MS:
		return true
	default:
		return false
	}
}

func (s *service) appendDerivedMetricsFromTraces(
	_ context.Context,
	input GetMetricCatalogInput,
	metrics []models.MetricMeta,
) ([]models.MetricMeta, apperr.Error) {
	seen := make(map[string]struct{}, len(metrics))
	for _, metric := range metrics {
		seen[metric.Name] = struct{}{}
	}

	searchLower := strings.ToLower(strings.TrimSpace(input.SearchQuery))
	for _, candidate := range derivedMetricCatalog {
		if searchLower != "" {
			nameMatch := strings.Contains(strings.ToLower(candidate.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(candidate.Description), searchLower)
			if !nameMatch && !descMatch {
				continue
			}
		}
		if _, exists := seen[candidate.Name]; exists {
			continue
		}

		if input.ServiceName != "" {
			candidate.ServiceName = input.ServiceName
		}
		metrics = append(metrics, candidate)
		seen[candidate.Name] = struct{}{}
	}

	return metrics, nil
}

func (s *service) queryDerivedTimeseries(ctx context.Context, input QueryTimeseriesInput) (*QueryTimeseriesOutput, apperr.Error) {
	if !isDerivedMetricName(input.MetricName) {
		return nil, errs.ErrBadRequest
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
	if bucketSeconds <= 0 {
		bucketSeconds = 60
	}

	valueExpr, ok := derivedMetricBucketExpr(input.MetricName, bucketSeconds)
	if !ok {
		return nil, errs.ErrBadRequest
	}

	args := &metricSQLBuilder{}
	hotWhere := buildDerivedTraceWhere("t", input, args)
	restoredWhere := buildDerivedTraceWhere("r", input, args)

	groupLabelKeys := make([]string, 0, len(input.GroupBy))
	groupExprs := make([]string, 0, len(input.GroupBy))
	for _, raw := range input.GroupBy {
		if expr, labelKey, ok := derivedGroupByExpr("u", raw); ok {
			groupExprs = append(groupExprs, expr)
			groupLabelKeys = append(groupLabelKeys, labelKey)
		}
	}

	labelSelect := ""
	labelGroupBy := ""
	for i, expr := range groupExprs {
		alias := fmt.Sprintf("g%d", i+1)
		labelSelect += fmt.Sprintf(", %s AS %s", expr, alias)
		labelGroupBy += ", " + alias
	}

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.service_name,
			t.deployment_environment,
			t.start_time,
			t.duration_ns,
			t.status_code,
			t.kind
		FROM traces_hot t
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.service_name,
			r.deployment_environment,
			r.start_time,
			r.duration_ns,
			r.status_code,
			r.kind
		FROM traces_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM traces_hot h
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
		hotWhere,
		restoredWhere,
		bucketExpression("u.start_time", step),
		valueExpr,
		labelSelect,
		labelGroupBy,
	)

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "queryDerivedTimeseries: query failed", "error", err)
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
		groupVals := make([]string, len(groupLabelKeys))
		scanArgs := []any{&bucket, &value}
		for i := range groupLabelKeys {
			scanArgs = append(scanArgs, &groupVals[i])
		}

		if err := rows.Scan(scanArgs...); err != nil {
			s.logger.ErrorContext(ctx, "queryDerivedTimeseries: scan failed", "error", err)
			return nil, errs.ErrInternal
		}

		labels := make(map[string]string, len(groupLabelKeys))
		keyParts := make([]string, 0, len(groupLabelKeys))
		for i, labelKey := range groupLabelKeys {
			labels[labelKey] = groupVals[i]
			keyParts = append(keyParts, fmt.Sprintf("%s=%s", labelKey, groupVals[i]))
		}
		key := strings.Join(keyParts, ",")

		ts, exists := seriesMap[seriesKey(key)]
		if !exists {
			ts = &models.Timeseries{Labels: labels}
			seriesMap[seriesKey(key)] = ts
		}
		ts.Points = append(ts.Points, models.TimeseriesPoint{Time: bucket, Value: value})
	}

	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "queryDerivedTimeseries: rows iteration error", "error", err)
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

func (s *service) queryDerivedSummary(ctx context.Context, input GetMetricSummaryInput) (*GetMetricSummaryOutput, apperr.Error) {
	if !isDerivedMetricName(input.MetricName) {
		return nil, errs.ErrBadRequest
	}

	lookback := input.LookbackWindow
	if lookback == 0 {
		lookback = 5 * time.Minute
	}
	now := time.Now()
	since := now.Add(-lookback)

	valueExpr, ok := derivedMetricSummaryExpr(input.MetricName, lookback.Seconds())
	if !ok {
		return nil, errs.ErrBadRequest
	}

	args := &metricSQLBuilder{}
	hotWhere := buildDerivedSummaryWhere("t", input, since, now, args)
	restoredWhere := buildDerivedSummaryWhere("r", input, since, now, args)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.service_name,
			t.deployment_environment,
			t.start_time,
			t.duration_ns,
			t.status_code,
			t.kind
		FROM traces_hot t
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.service_name,
			r.deployment_environment,
			r.start_time,
			r.duration_ns,
			r.status_code,
			r.kind
		FROM traces_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM traces_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
		coalesce(%s, 0)::DOUBLE PRECISION AS value,
		coalesce(max(start_time), NOW()) AS latest
	FROM unioned u`,
		hotWhere,
		restoredWhere,
		valueExpr,
	)

	var out GetMetricSummaryOutput
	if err := s.pg.Pool().QueryRow(ctx, query, args.args...).Scan(&out.Value, &out.Timestamp); err != nil {
		s.logger.ErrorContext(ctx, "queryDerivedSummary: query failed", "error", err)
		return nil, errs.ErrInternal
	}
	return &out, nil
}

func buildDerivedTraceWhere(alias string, input QueryTimeseriesInput, args *metricSQLBuilder) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
		fmt.Sprintf("%sstart_time >= %s", prefix, args.add(input.StartTime)),
		fmt.Sprintf("%sstart_time <= %s", prefix, args.add(input.EndTime)),
		fmt.Sprintf("%skind = 2", prefix), // server spans
	}

	if input.ServiceName != "" {
		clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(input.ServiceName)))
	}
	clauses = append(clauses, buildDerivedFilterClauses(prefix, input.Filters, args)...)

	return strings.Join(clauses, " AND ")
}

func buildDerivedSummaryWhere(
	alias string,
	input GetMetricSummaryInput,
	start time.Time,
	end time.Time,
	args *metricSQLBuilder,
) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(input.OrganizationID)),
		fmt.Sprintf("%sstart_time >= %s", prefix, args.add(start)),
		fmt.Sprintf("%sstart_time <= %s", prefix, args.add(end)),
		fmt.Sprintf("%skind = 2", prefix), // server spans
	}

	if input.ServiceName != "" {
		clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(input.ServiceName)))
	}
	clauses = append(clauses, buildDerivedFilterClauses(prefix, input.Filters, args)...)

	return strings.Join(clauses, " AND ")
}

func buildDerivedFilterClauses(prefix string, filters map[string]string, args *metricSQLBuilder) []string {
	if len(filters) == 0 {
		return nil
	}

	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	clauses := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(filters[key])
		if value == "" {
			continue
		}
		switch normalizeDerivedField(key) {
		case "service_name":
			clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(value)))
		case "deployment_environment":
			clauses = append(clauses, fmt.Sprintf("%sdeployment_environment = %s", prefix, args.add(value)))
		case "status_code":
			clauses = append(clauses, fmt.Sprintf("%sstatus_code::text = %s", prefix, args.add(value)))
		case "kind":
			clauses = append(clauses, fmt.Sprintf("%skind::text = %s", prefix, args.add(value)))
		}
	}
	return clauses
}

func derivedGroupByExpr(alias string, raw string) (expr string, labelKey string, ok bool) {
	switch normalizeDerivedField(raw) {
	case "service_name":
		return alias + ".service_name", "service_name", true
	case "deployment_environment":
		return alias + ".deployment_environment", "deployment_environment", true
	case "status_code":
		return alias + ".status_code::text", "status_code", true
	case "kind":
		return alias + ".kind::text", "kind", true
	default:
		return "", "", false
	}
}

func normalizeDerivedField(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "service", "service_name":
		return "service_name"
	case "deployment.environment", "deployment_environment":
		return "deployment_environment"
	case "status", "status_code":
		return "status_code"
	case "kind":
		return "kind"
	default:
		return k
	}
}

func derivedMetricBucketExpr(metricName string, bucketSeconds int) (string, bool) {
	seconds := bucketSeconds
	if seconds <= 0 {
		seconds = 60
	}

	switch strings.ToLower(strings.TrimSpace(metricName)) {
	case derivedMetricRequestsRate:
		return fmt.Sprintf("count(*)::DOUBLE PRECISION / %d", seconds), true
	case derivedMetricRequestsTotal:
		return "count(*)::DOUBLE PRECISION", true
	case derivedMetricErrorsRate:
		return fmt.Sprintf("count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION / %d", seconds), true
	case derivedMetricErrorsTotal:
		return "count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION", true
	case derivedMetricErrorsRatioPct:
		return "CASE WHEN count(*) = 0 THEN 0 ELSE 100.0 * count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION / count(*)::DOUBLE PRECISION END", true
	case derivedMetricLatencyAvgMS:
		return "avg(u.duration_ns::DOUBLE PRECISION / 1000000.0)", true
	case derivedMetricLatencyP50MS:
		return "percentile_cont(0.5) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP90MS:
		return "percentile_cont(0.9) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP95MS:
		return "percentile_cont(0.95) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP99MS:
		return "percentile_cont(0.99) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	default:
		return "", false
	}
}

func derivedMetricSummaryExpr(metricName string, lookbackSeconds float64) (string, bool) {
	seconds := lookbackSeconds
	if seconds <= 0 {
		seconds = 300
	}

	switch strings.ToLower(strings.TrimSpace(metricName)) {
	case derivedMetricRequestsRate:
		return fmt.Sprintf("count(*)::DOUBLE PRECISION / %f", seconds), true
	case derivedMetricRequestsTotal:
		return "count(*)::DOUBLE PRECISION", true
	case derivedMetricErrorsRate:
		return fmt.Sprintf("count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION / %f", seconds), true
	case derivedMetricErrorsTotal:
		return "count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION", true
	case derivedMetricErrorsRatioPct:
		return "CASE WHEN count(*) = 0 THEN 0 ELSE 100.0 * count(*) FILTER (WHERE u.status_code = 2)::DOUBLE PRECISION / count(*)::DOUBLE PRECISION END", true
	case derivedMetricLatencyAvgMS:
		return "avg(u.duration_ns::DOUBLE PRECISION / 1000000.0)", true
	case derivedMetricLatencyP50MS:
		return "percentile_cont(0.5) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP90MS:
		return "percentile_cont(0.9) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP95MS:
		return "percentile_cont(0.95) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	case derivedMetricLatencyP99MS:
		return "percentile_cont(0.99) WITHIN GROUP (ORDER BY (u.duration_ns::DOUBLE PRECISION / 1000000.0))", true
	default:
		return "", false
	}
}
