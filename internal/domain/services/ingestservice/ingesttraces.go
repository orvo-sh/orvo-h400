package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
	tracespb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type IngestTracesInput struct {
	OrganizationID string
	ResourceSpans  []*tracespb.ResourceSpans
}

func (s *service) IngestTraces(ctx context.Context, input IngestTracesInput) apperr.Error {
	s.logger.InfoContext(ctx, "IngestTraces: ingesting traces",
		slog.String("organization_id", input.OrganizationID),
		slog.Int("resource_spans_count", len(input.ResourceSpans)),
	)

	records := s.transformTraces(input.ResourceSpans, input.OrganizationID)
	if len(records) == 0 {
		return nil
	}

	if err := s.spanSink.Enqueue(ctx, records); err != nil {
		s.logger.ErrorContext(ctx, "IngestTraces: failed to enqueue spans", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
