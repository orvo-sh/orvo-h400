package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

func (s *service) GetSession(ctx context.Context, token string) (*models.Session, apperr.Error) {
	s.logger.InfoContext(ctx, "GetSession: getting session", slog.String("token", token))

	row, err := s.postgres.Queries.GetSessionByToken(ctx, token)
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrSessionNotFound
		}
		s.logger.ErrorContext(ctx, "GetSession: failed to get session", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	if row.ExpiresAt.Time.Before(time.Now()) {
		return nil, errs.ErrSessionExpired
	}

	return &models.Session{
		ID:                   row.ID,
		Token:                row.Token,
		UserID:               row.UserID,
		ActiveOrganizationID: pgutil.TextToPtr(row.ActiveOrganizationID),
		IpAddress:            pgutil.TextToPtr(row.IpAddress),
		UserAgent:            pgutil.TextToPtr(row.UserAgent),
		ExpiresAt:            row.ExpiresAt.Time,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}, nil
}
