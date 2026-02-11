package authservice

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/sensitive"
	"github.com/orvo-sh/orvo/pkg/util"
)

type LoginInput struct {
	Email     string
	Password  sensitive.Sensitive[string]
	IpAddress *string
	UserAgent *string
}

func (s *service) Login(ctx context.Context, input LoginInput) (*models.Session, apperr.Error) {
	s.logger.InfoContext(ctx, "Login: logging in user", slog.Any("input", input))

	user, err := s.postgres.Queries.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrInvalidCredentials
		}
		s.logger.ErrorContext(ctx, "Login: failed to get user", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	account, err := s.postgres.Queries.GetAccountByProvider(ctx, db.GetAccountByProviderParams{
		Provider:          string(models.AccountProviderEmail),
		ProviderAccountID: input.Email,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, errs.ErrInvalidCredentials
		}
		s.logger.ErrorContext(ctx, "Login: failed to get account", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	if !account.PasswordHash.Valid {
		return nil, errs.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash.String), []byte(input.Password.Value())); err != nil {
		return nil, errs.ErrInvalidCredentials
	}

	dbSession, err := s.postgres.Queries.CreateSession(ctx, db.CreateSessionParams{
		ID:                   util.GenerateID("ses"),
		Token:                util.GenerateRandomString(),
		UserID:               user.ID,
		ActiveOrganizationID: pgutil.NullText(),
		IpAddress:            pgutil.Text(input.IpAddress),
		UserAgent:            pgutil.Text(input.UserAgent),
		ExpiresAt:            pgutil.Timestamptz(time.Now().Add(s.config.SessionExpiresIn)),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "Login: failed to create session", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &models.Session{
		ID:                   dbSession.ID,
		Token:                dbSession.Token,
		UserID:               dbSession.UserID,
		ActiveOrganizationID: pgutil.TextToPtr(dbSession.ActiveOrganizationID),
		IpAddress:            pgutil.TextToPtr(dbSession.IpAddress),
		UserAgent:            pgutil.TextToPtr(dbSession.UserAgent),
		ExpiresAt:            dbSession.ExpiresAt.Time,
		CreatedAt:            dbSession.CreatedAt.Time,
		UpdatedAt:            dbSession.UpdatedAt.Time,
	}, nil
}
