package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

func (s *service) ResolveApiKey(ctx context.Context, rawKey string) (*string, apperr.Error) {
	s.logger.InfoContext(ctx, "ResolveApiKey: resolving API key")

	keyHash := hashKey(rawKey)

	s.apiKeyResolverMu.RLock()
	if entry, ok := s.apiKeyResolverCache[keyHash]; ok && time.Now().Before(entry.expiresAt) {
		s.apiKeyResolverMu.RUnlock()
		return &entry.organizationID, nil
	}
	s.apiKeyResolverMu.RUnlock()

	apiKey, err := s.postgres.Queries.GetApiKeyByHash(ctx, keyHash)
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrApiKeyNotFound
		}
		s.logger.ErrorContext(ctx, "ResolveApiKey: failed to get API key by hash", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	s.apiKeyResolverMu.Lock()
	s.apiKeyResolverCache[keyHash] = apiKeyCacheEntry{
		organizationID: apiKey.OrganizationID,
		expiresAt:      time.Now().Add(s.config.ApiKeyCacheResolverTTL),
	}
	s.apiKeyResolverMu.Unlock()

	s.backgroundManager.Run(func(ctx context.Context) {
		if err := s.postgres.Queries.UpdateApiKeyLastUsed(context.Background(), keyHash); err != nil {
			s.logger.Error("ResolveApiKey: failed to update last_used_at", slog.Any("error", err))
		}
	})

	return &apiKey.OrganizationID, nil
}
