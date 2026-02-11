package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

type IngestMetricsInput struct {
	OrganizationID  string
	ResourceMetrics []*metricspb.ResourceMetrics
}

func (s *service) IngestMetrics(ctx context.Context, input IngestMetricsInput) apperr.Error {
	s.logger.InfoContext(ctx, "IngestMetrics: ingesting metrics",
		slog.String("organization_id", input.OrganizationID),
		slog.Int("resource_metrics_count", len(input.ResourceMetrics)),
	)

	points := s.transformMetrics(input.ResourceMetrics, input.OrganizationID)
	if len(points) == 0 {
		return nil
	}

	if err := s.metricSink.Enqueue(ctx, points); err != nil {
		s.logger.ErrorContext(ctx, "IngestMetrics: failed to enqueue metrics", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
