package authservice

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type RegisterInput struct {
	Email                string
	Password             string
	Name                 string
	IpAddress            *string
	UserAgent            *string
	ActiveOrganizationID *string
}

func (s *service) Register(ctx context.Context, input RegisterInput) (*models.Session, apperr.Error) {
	s.logger.InfoContext(ctx, "Register: registering user", slog.Any("input", input))

	var session models.Session

	err := s.postgres.WithTx(ctx, func(q *postgres.Queries) error {
		user, err := s.postgres.Queries.CreateUser(ctx, db.CreateUserParams{
			ID:            util.GenerateID("usr"),
			Email:         input.Email,
			EmailVerified: false,
			Name:          input.Name,
		})
		if err != nil {
			if pgutil.IsUniqueViolationError(err, []string{"email"}) {
				return errs.ErrEmailAlreadyExists
			}
			s.logger.ErrorContext(ctx, "Register: failed to create user", slog.Any("error", err))
			return errs.ErrInternal
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			s.logger.ErrorContext(ctx, "Register: failed to hash password", slog.Any("error", err))
			return errs.ErrInternal
		}

		if _, err = s.postgres.Queries.CreateAccount(ctx, db.CreateAccountParams{
			ID:                util.GenerateID("acc"),
			UserID:            user.ID,
			Provider:          string(models.AccountProviderEmail),
			ProviderAccountID: input.Email,
			PasswordHash:      pgutil.TextFromString(string(hashedPassword)),
		}); err != nil {
			s.logger.ErrorContext(ctx, "Register: failed to create account", slog.Any("error", err))
			return errs.ErrInternal
		}

		dbSession, err := s.postgres.Queries.CreateSession(ctx, db.CreateSessionParams{
			ID:                   util.GenerateID("ses"),
			Token:                util.GenerateRandomString(),
			UserID:               user.ID,
			ActiveOrganizationID: pgutil.Text(input.ActiveOrganizationID),
			IpAddress:            pgutil.Text(input.IpAddress),
			UserAgent:            pgutil.Text(input.UserAgent),
			ExpiresAt:            pgutil.Timestamptz(time.Now().Add(s.config.SessionExpiresIn)),
		})
		if err != nil {
			s.logger.ErrorContext(ctx, "Register: failed to create session", slog.Any("error", err))
			return errs.ErrInternal
		}
		session = models.Session{
			ID:                   dbSession.ID,
			Token:                dbSession.Token,
			UserID:               dbSession.UserID,
			ActiveOrganizationID: pgutil.TextToPtr(dbSession.ActiveOrganizationID),
			IpAddress:            pgutil.TextToPtr(dbSession.IpAddress),
			UserAgent:            pgutil.TextToPtr(dbSession.UserAgent),
			ExpiresAt:            dbSession.ExpiresAt.Time,
			CreatedAt:            dbSession.CreatedAt.Time,
			UpdatedAt:            dbSession.UpdatedAt.Time,
		}

		return nil
	})

	if err != nil {
		if appErr, ok := err.(apperr.Error); ok {
			return nil, appErr
		}
		s.logger.ErrorContext(ctx, "Register: transaction failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &session, nil
}
