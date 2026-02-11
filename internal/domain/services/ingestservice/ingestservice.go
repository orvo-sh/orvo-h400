package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/sink"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	IngestLogs(ctx context.Context, input IngestLogsInput) apperr.Error
}

type service struct {
	sink   *sink.LogSink
	logger *slog.Logger
}

func New(sink *sink.LogSink, logger *slog.Logger) Service {
	return &service{
		sink:   sink,
		logger: logger,
	}
}
