package ingestservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type IngestLogInput struct {
	Timestamp      time.Time
	Level          models.LogLevel
	Message        string
	Service        string
	Environment    string
	OrganizationID string
	ParentID       string
	Attributes     map[string]any
}

func (s *service) IngestLogEvent(ctx context.Context, input IngestLogInput) (*models.LogEvent, apperr.Error) {
	s.logger.Info("IngestLogEvent: ingesting log event", slog.Any("input", input))

	s.redisClient.RPush(ctx, "logs_queue", input)

	return nil, nil
}
