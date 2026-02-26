package metricservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetREDMetrics(ctx context.Context, input GetREDMetricsInput) (*models.REDMetrics, apperr.Error) {
	s.logger.InfoContext(ctx, "GetREDMetrics",
		slog.String("organization_id", input.OrganizationID),
		slog.String("service_name", input.ServiceName),
	)

	if input.StartTime.IsZero() {
		input.StartTime = time.Now().Add(-1 * time.Hour)
	}
	if input.EndTime.IsZero() {
		input.EndTime = time.Now()
	}

	step := normalizeStep(input.StartTime, input.EndTime, input.Step)

	requestRate, err := s.queryREDSeries(ctx, input, derivedMetricRequestsRate, "rate", step)
	if err != nil {
		return nil, err
	}
	errorRate, err := s.queryREDSeries(ctx, input, derivedMetricErrorsRate, "rate", step)
	if err != nil {
		return nil, err
	}
	p50, err := s.queryREDSeries(ctx, input, derivedMetricLatencyP50MS, "p50", step)
	if err != nil {
		return nil, err
	}
	p90, err := s.queryREDSeries(ctx, input, derivedMetricLatencyP90MS, "p90", step)
	if err != nil {
		return nil, err
	}
	p95, err := s.queryREDSeries(ctx, input, derivedMetricLatencyP95MS, "p95", step)
	if err != nil {
		return nil, err
	}
	p99, err := s.queryREDSeries(ctx, input, derivedMetricLatencyP99MS, "p99", step)
	if err != nil {
		return nil, err
	}

	return &models.REDMetrics{
		RequestRate: requestRate,
		ErrorRate:   errorRate,
		P50Latency:  p50,
		P90Latency:  p90,
		P95Latency:  p95,
		P99Latency:  p99,
	}, nil
}

func (s *service) queryREDSeries(
	ctx context.Context,
	input GetREDMetricsInput,
	metricName string,
	aggregation string,
	step string,
) ([]models.TimeseriesPoint, apperr.Error) {
	output, err := s.QueryTimeseries(ctx, QueryTimeseriesInput{
		OrganizationID: input.OrganizationID,
		MetricName:     metricName,
		StartTime:      input.StartTime,
		EndTime:        input.EndTime,
		Step:           step,
		Aggregation:    aggregation,
		ServiceName:    input.ServiceName,
	})
	if err != nil {
		return nil, err
	}
	if len(output.Series) == 0 {
		return []models.TimeseriesPoint{}, nil
	}
	return output.Series[0].Points, nil
}
