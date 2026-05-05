package metricservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) RecalculateDerivedMetrics(
	ctx context.Context,
	input RecalculateDerivedMetricsInput,
) (*RecalculateDerivedMetricsOutput, apperr.Error) {
	if strings.TrimSpace(input.OrganizationID) == "" {
		return nil, errs.ErrBadRequest
	}

	lookback := input.LookbackWindow
	if lookback <= 0 {
		lookback = time.Hour
	}

	windowEnd := time.Now().UTC()
	windowStart := windowEnd.Add(-lookback)

	requestSpanCount, errorSpanCount, latestTraceTime, err := s.queryTraceRecalculationStats(
		ctx,
		input.OrganizationID,
		input.ServiceName,
		windowStart,
		windowEnd,
	)
	if err != nil {
		s.logger.ErrorContext(ctx, "RecalculateDerivedMetrics: trace stats query failed", "error", err)
		return nil, errs.ErrInternal
	}

	errorLogCount, latestLogTime, err := s.queryLogRecalculationStats(
		ctx,
		input.OrganizationID,
		input.ServiceName,
		windowStart,
		windowEnd,
	)
	if err != nil {
		s.logger.ErrorContext(ctx, "RecalculateDerivedMetrics: log stats query failed", "error", err)
		return nil, errs.ErrInternal
	}

	asOf := latestTraceTime
	if latestLogTime.After(asOf) {
		asOf = latestLogTime
	}

	return &RecalculateDerivedMetricsOutput{
		OrganizationID:      input.OrganizationID,
		ServiceName:         input.ServiceName,
		WindowStart:         windowStart,
		WindowEnd:           windowEnd,
		AsOf:                asOf,
		RequestSpanCount:    requestSpanCount,
		ErrorSpanCount:      errorSpanCount,
		ErrorLogCount:       errorLogCount,
		CombinedErrorEvents: errorSpanCount + errorLogCount,
	}, nil
}

func (s *service) queryTraceRecalculationStats(
	ctx context.Context,
	organizationID string,
	serviceName string,
	windowStart time.Time,
	windowEnd time.Time,
) (int64, int64, time.Time, error) {
	args := &metricSQLBuilder{}
	hotWhere := buildTraceRecalculationWhere("t", organizationID, serviceName, windowStart, windowEnd, args)
	restoredWhere := buildTraceRecalculationWhere("r", organizationID, serviceName, windowStart, windowEnd, args)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.start_time,
			t.status_code
		FROM traces_hot t
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.start_time,
			r.status_code
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
		count(*)::BIGINT AS request_span_count,
		count(*) FILTER (WHERE status_code = 2)::BIGINT AS error_span_count,
		coalesce(max(start_time), %s::timestamptz) AS latest_trace_time
	FROM unioned`,
		hotWhere,
		restoredWhere,
		args.add(windowStart),
	)

	var requestSpanCount int64
	var errorSpanCount int64
	var latestTraceTime time.Time
	if err := s.pg.Pool().QueryRow(ctx, query, args.args...).Scan(&requestSpanCount, &errorSpanCount, &latestTraceTime); err != nil {
		return 0, 0, time.Time{}, err
	}

	return requestSpanCount, errorSpanCount, latestTraceTime, nil
}

func (s *service) queryLogRecalculationStats(
	ctx context.Context,
	organizationID string,
	serviceName string,
	windowStart time.Time,
	windowEnd time.Time,
) (int64, time.Time, error) {
	args := &metricSQLBuilder{}
	hotWhere := buildLogRecalculationWhere("l", organizationID, serviceName, windowStart, windowEnd, args)
	restoredWhere := buildLogRecalculationWhere("r", organizationID, serviceName, windowStart, windowEnd, args)

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			l.id,
			l.organization_id,
			l.timestamp,
			l.severity_number,
			l.severity_text
		FROM logs_hot l
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.timestamp,
			r.severity_number,
			r.severity_text
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
		count(*)::BIGINT AS error_log_count,
		coalesce(max(timestamp), %s::timestamptz) AS latest_log_time
	FROM unioned u
	WHERE
		u.severity_number >= 17
		OR lower(coalesce(u.severity_text, '')) IN ('error', 'fatal')`,
		hotWhere,
		restoredWhere,
		args.add(windowStart),
	)

	var errorLogCount int64
	var latestLogTime time.Time
	if err := s.pg.Pool().QueryRow(ctx, query, args.args...).Scan(&errorLogCount, &latestLogTime); err != nil {
		return 0, time.Time{}, err
	}

	return errorLogCount, latestLogTime, nil
}

func buildTraceRecalculationWhere(
	alias string,
	organizationID string,
	serviceName string,
	windowStart time.Time,
	windowEnd time.Time,
	args *metricSQLBuilder,
) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(organizationID)),
		fmt.Sprintf("%sstart_time >= %s", prefix, args.add(windowStart)),
		fmt.Sprintf("%sstart_time <= %s", prefix, args.add(windowEnd)),
		fmt.Sprintf("%skind = 2", prefix), // server spans
	}
	if serviceName != "" {
		clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(serviceName)))
	}
	return strings.Join(clauses, " AND ")
}

func buildLogRecalculationWhere(
	alias string,
	organizationID string,
	serviceName string,
	windowStart time.Time,
	windowEnd time.Time,
	args *metricSQLBuilder,
) string {
	prefix := alias + "."
	clauses := []string{
		fmt.Sprintf("%sorganization_id = %s", prefix, args.add(organizationID)),
		fmt.Sprintf("%stimestamp >= %s", prefix, args.add(windowStart)),
		fmt.Sprintf("%stimestamp <= %s", prefix, args.add(windowEnd)),
	}
	if serviceName != "" {
		clauses = append(clauses, fmt.Sprintf("%sservice_name = %s", prefix, args.add(serviceName)))
	}
	return strings.Join(clauses, " AND ")
}
