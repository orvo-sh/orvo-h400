package ingest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/redis"
)

type Service struct {
	redisClient *redis.Client
}

func New(redisClient *redis.Client) *Service {
	return &Service{
		redisClient: redisClient,
	}
}

func (s *Service) IngestLog(ctx context.Context, log models.Log) error {
	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	return s.redisClient.RPush(ctx, "logs_queue", data)
}