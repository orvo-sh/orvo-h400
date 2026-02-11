package sink

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/pkg/batcher"
)

type MetricSink struct {
	clickhouse *clickhouse.DB
	batcher    *batcher.Batcher[models.MetricPoint]
}

func NewMetricSink(clickhouse *clickhouse.DB, logger *slog.Logger) *MetricSink {
	s := &MetricSink{
		clickhouse: clickhouse,
	}
	s.batcher = batcher.New(
		logger.With("module", "MetricSinkBatcher"),
		s.writeBatch,
		batcher.WithBatchSize(10000),
		batcher.WithFlushInterval(5*time.Second),
		batcher.WithMaxQueueSize(100000),
	)
	return s
}

func (s *MetricSink) Enqueue(_ context.Context, records []models.MetricPoint) error {
	for _, r := range records {
		if err := s.batcher.Push(r); err != nil {
			return fmt.Errorf("failed to enqueue metric: %w", err)
		}
	}
	return nil
}

func (s *MetricSink) Close() error {
	s.batcher.Close()
	return nil
}

func (s *MetricSink) writeBatch(ctx context.Context, records []models.MetricPoint) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := s.clickhouse.PrepareBatch(ctx, `INSERT INTO metrics (
		organization_id,
		metric_name, metric_type, metric_unit, metric_description,
		service_name, deployment_environment,
		resource_attributes,
		scope_name, scope_version,
		attributes,
		start_time, time,
		value_int, value_double,
		aggregation_temporality, is_monotonic,
		histogram_count, histogram_sum, histogram_min, histogram_max,
		histogram_bucket_counts, histogram_explicit_bounds,
		exemplar_trace_ids, exemplar_span_ids, exemplar_values, exemplar_timestamps,
		flags
	)`)
	if err != nil {
		return fmt.Errorf("failed to prepare metric batch: %w", err)
	}

	for _, r := range records {
		// Convert exemplars to parallel arrays.
		exemplarTraceIDs := make([]string, len(r.Exemplars))
		exemplarSpanIDs := make([]string, len(r.Exemplars))
		exemplarValues := make([]float64, len(r.Exemplars))
		exemplarTimestamps := make([]time.Time, len(r.Exemplars))
		for i, e := range r.Exemplars {
			exemplarTraceIDs[i] = e.TraceID
			exemplarSpanIDs[i] = e.SpanID
			exemplarValues[i] = e.Value
			exemplarTimestamps[i] = e.Timestamp
		}

		// Ensure resource attributes is not nil.
		resourceAttrs := r.ResourceAttrs
		if resourceAttrs == nil {
			resourceAttrs = map[string]string{}
		}

		// Ensure attributes is not nil.
		attrs := r.Attributes
		if attrs == nil {
			attrs = map[string]string{}
		}

		// Ensure histogram arrays are not nil (ClickHouse needs non-nil arrays).
		bucketCounts := r.HistogramBucketCounts
		if bucketCounts == nil {
			bucketCounts = []uint64{}
		}
		explicitBounds := r.HistogramExplicitBounds
		if explicitBounds == nil {
			explicitBounds = []float64{}
		}

		if err := batch.Append(
			r.OrganizationID,
			r.MetricName, r.MetricType, r.MetricUnit, r.Description,
			r.ServiceName, r.DeploymentEnv,
			resourceAttrs,
			r.ScopeName, r.ScopeVersion,
			attrs,
			r.StartTime, r.Time,
			r.ValueInt, r.ValueDouble,
			r.AggregationTemporality, r.IsMonotonic,
			r.HistogramCount, r.HistogramSum, r.HistogramMin, r.HistogramMax,
			bucketCounts, explicitBounds,
			exemplarTraceIDs, exemplarSpanIDs, exemplarValues, exemplarTimestamps,
			r.Flags,
		); err != nil {
			return fmt.Errorf("failed to append metric to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send metric batch: %w", err)
	}

	return nil
}
