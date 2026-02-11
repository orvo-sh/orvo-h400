package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/sink"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	IngestLogs(ctx context.Context, input IngestLogsInput) apperr.Error
	IngestTraces(ctx context.Context, input IngestTracesInput) apperr.Error
}

type service struct {
	logSink  *sink.LogSink
	spanSink *sink.SpanSink
	logger   *slog.Logger
}

func New(logSink *sink.LogSink, spanSink *sink.SpanSink, logger *slog.Logger) Service {
	return &service{
		logSink:  logSink,
		spanSink: spanSink,
		logger:   logger,
	}
}
