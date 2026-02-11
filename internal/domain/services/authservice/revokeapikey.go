package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

type RevokeApiKeyInput struct {
	ID             string
	OrganizationID string
}

func (s *service) RevokeApiKey(ctx context.Context, input RevokeApiKeyInput) apperr.Error {
	s.logger.InfoContext(ctx, "RevokeApiKey: revoking API key", slog.Any("input", input))

	_, err := s.postgres.Queries.RevokeApiKey(ctx, db.RevokeApiKeyParams{
		ID:             input.ID,
		OrganizationID: input.OrganizationID,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return errs.ErrApiKeyNotFound
		}
		s.logger.ErrorContext(ctx, "RevokeApiKey: failed to revoke API key", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
