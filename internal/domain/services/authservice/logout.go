package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) Logout(ctx context.Context, token string) apperr.Error {
	s.logger.InfoContext(ctx, "Logout: logging out session")

	err := s.postgres.Queries.DeleteSessionByToken(ctx, token)
	if err != nil {
		s.logger.ErrorContext(ctx, "Logout: failed to delete session", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
