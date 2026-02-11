package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

type IngestLogsInput struct {
	OrganizationID string
	ResourceLogs   []*logspb.ResourceLogs
}

func (s *service) IngestLogs(ctx context.Context, input IngestLogsInput) apperr.Error {
	s.logger.InfoContext(ctx, "IngestLogs: ingesting logs",
		slog.String("organization_id", input.OrganizationID),
		slog.Int("resource_logs_count", len(input.ResourceLogs)),
	)

	records := s.transformLogs(input.ResourceLogs, input.OrganizationID)
	if len(records) == 0 {
		return nil
	}

	if err := s.logSink.Enqueue(ctx, records); err != nil {
		s.logger.ErrorContext(ctx, "IngestLogs: failed to enqueue logs", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
