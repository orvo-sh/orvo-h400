package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

func (s *service) ListApiKeys(ctx context.Context, organizationID string) ([]models.ApiKey, apperr.Error) {
	s.logger.InfoContext(ctx, "ListApiKeys: listing API keys", slog.String("organization_id", organizationID))

	dbKeys, err := s.postgres.Queries.ListApiKeysByOrganizationID(ctx, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "ListApiKeys: failed to list API keys", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	keys := make([]models.ApiKey, len(dbKeys))
	for i, k := range dbKeys {
		keys[i] = models.ApiKey{
			ID:             k.ID,
			OrganizationID: k.OrganizationID,
			KeyHash:        k.KeyHash,
			Name:           k.Name,
			LastUsedAt:     pgutil.TimestamptzToPtr(k.LastUsedAt),
			ExpiresAt:      pgutil.TimestamptzToPtr(k.ExpiresAt),
			CreatedAt:      pgutil.TimestamptzToTime(k.CreatedAt),
			RevokedAt:      pgutil.TimestamptzToPtr(k.RevokedAt),
		}
	}

	return keys, nil
}
