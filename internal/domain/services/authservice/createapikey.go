package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type CreateApiKeyInput struct {
	OrganizationID string
	Name           string
	ExpiresIn      *time.Duration
}

func (s *service) CreateApiKey(ctx context.Context, input CreateApiKeyInput) (*models.ApiKey, *string, apperr.Error) {
	s.logger.InfoContext(ctx, "CreateApiKey: creating API key", slog.Any("input", input))

	rawKey := util.GenerateRandomString(48)
	keyHash := hashKey(rawKey)

	expiresAt := pgutil.NullTimestamptz()
	if input.ExpiresIn != nil {
		expiresAt = pgutil.Timestamptz(time.Now().Add(*input.ExpiresIn))
	}

	dbKey, err := s.postgres.Queries.CreateApiKey(ctx, db.CreateApiKeyParams{
		ID:             util.GenerateID("key"),
		OrganizationID: input.OrganizationID,
		KeyHash:        keyHash,
		Name:           input.Name,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "CreateApiKey: failed to create API key", slog.Any("error", err))
		return nil, nil, errs.ErrInternal
	}

	return &models.ApiKey{
		ID:             dbKey.ID,
		OrganizationID: dbKey.OrganizationID,
		KeyHash:        dbKey.KeyHash,
		Name:           dbKey.Name,
		LastUsedAt:     pgutil.TimestamptzToPtr(dbKey.LastUsedAt),
		ExpiresAt:      pgutil.TimestamptzToPtr(dbKey.ExpiresAt),
		CreatedAt:      pgutil.TimestamptzToTime(dbKey.CreatedAt),
		RevokedAt:      pgutil.TimestamptzToPtr(dbKey.RevokedAt),
	}, &rawKey, nil
}
