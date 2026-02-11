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
	IngestMetrics(ctx context.Context, input IngestMetricsInput) apperr.Error
}

type service struct {
	logSink    *sink.LogSink
	spanSink   *sink.SpanSink
	metricSink *sink.MetricSink
	logger     *slog.Logger
}

func New(logSink *sink.LogSink, spanSink *sink.SpanSink, metricSink *sink.MetricSink, logger *slog.Logger) Service {
	return &service{
		logSink:    logSink,
		spanSink:   spanSink,
		metricSink: metricSink,
		logger:     logger,
	}
}
