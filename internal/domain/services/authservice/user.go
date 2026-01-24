package authservice

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

// GetUser retrieves a user by ID
func (s *service) GetUser(ctx context.Context, userID string) (*User, apperr.Error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		s.logger.Error("failed to get user", "error", err)
		return nil, apperr.ErrInternal
	}

	return &User{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		Image:         pgutil.TextToPtr(user.Image),
		CreatedAt:     pgutil.TimestamptzToTime(user.CreatedAt),
	}, nil
}

type UpdateUserInput struct {
	UserID        string
	Name          *string
	Email         *string
	EmailVerified *bool
	Image         *string
}

// UpdateUser updates user information
func (s *service) UpdateUser(ctx context.Context, input UpdateUserInput) (*User, apperr.Error) {
	user, err := s.queries.UpdateUser(ctx, db.UpdateUserParams{
		ID:            input.UserID,
		Name:          pgutil.Text(input.Name),
		Email:         pgutil.Text(input.Email),
		EmailVerified: pgutil.BoolFromPtr(input.EmailVerified),
		Image:         pgutil.Text(input.Image),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		s.logger.Error("failed to update user", "error", err)
		return nil, apperr.ErrInternal
	}

	return &User{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		Image:         pgutil.TextToPtr(user.Image),
		CreatedAt:     pgutil.TimestamptzToTime(user.CreatedAt),
	}, nil
}
