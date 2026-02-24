package sink

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/batcher"
	"github.com/orvo-sh/orvo/pkg/util"
)

type MetricSink struct {
	postgres *postgres.DB
	batcher  *batcher.Batcher[models.MetricPoint]
}

func NewMetricSink(postgres *postgres.DB, logger *slog.Logger) *MetricSink {
	s := &MetricSink{
		postgres: postgres,
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

	const query = `INSERT INTO metrics_hot (
		id,
		organization_id,
		metric_name,
		metric_type,
		metric_unit,
		metric_description,
		service_name,
		deployment_environment,
		resource_attributes,
		scope_name,
		scope_version,
		attributes,
		start_time,
		time,
		value_int,
		value_double,
		aggregation_temporality,
		is_monotonic,
		histogram_count,
		histogram_sum,
		histogram_min,
		histogram_max,
		histogram_bucket_counts,
		histogram_explicit_bounds,
		exemplars,
		flags
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12::jsonb,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25::jsonb,$26
	)`

	var dbBatch pgx.Batch

	for _, r := range records {
		metricID := r.ID
		if metricID == "" {
			metricID = util.GenerateID("met")
		}

		resourceAttrs, err := jsonObject(r.ResourceAttrs)
		if err != nil {
			return fmt.Errorf("marshal resource attributes: %w", err)
		}
		attrs, err := jsonObject(r.Attributes)
		if err != nil {
			return fmt.Errorf("marshal metric attributes: %w", err)
		}
		exemplars, err := jsonArray(r.Exemplars)
		if err != nil {
			return fmt.Errorf("marshal metric exemplars: %w", err)
		}

		bucketCounts := r.HistogramBucketCounts
		if bucketCounts == nil {
			bucketCounts = []uint64{}
		}
		explicitBounds := r.HistogramExplicitBounds
		if explicitBounds == nil {
			explicitBounds = []float64{}
		}

		dbBatch.Queue(
			query,
			metricID,
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
			exemplars,
			r.Flags,
		)
	}

	results := s.postgres.Pool().SendBatch(ctx, &dbBatch)
	for range records {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert metrics_hot batch: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close metrics_hot batch: %w", err)
	}

	return nil
}
