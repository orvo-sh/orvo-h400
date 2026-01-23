package ingestservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/redis"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	IngestLogEvent(ctx context.Context, input IngestLogInput) (*models.LogEvent, apperr.Error)
}

type service struct {
	redisClient *redis.Client

	logger *slog.Logger
}

func New(logger *slog.Logger, redisClient *redis.Client) Service {
	return &service{
		redisClient: redisClient,
	}
}
