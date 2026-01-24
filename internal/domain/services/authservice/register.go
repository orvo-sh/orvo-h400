package authservice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

var (
	ErrInvalidCredentials = apperr.New(401, "invalid_credentials")
	ErrEmailAlreadyExists = apperr.New(409, "email_already_exists")
	ErrSessionNotFound    = apperr.New(401, "session_not_found")
	ErrSessionExpired     = apperr.New(401, "session_expired")
	ErrUserNotFound       = apperr.New(404, "user_not_found")
)

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

func (s *service) Register(ctx context.Context, input RegisterInput) (*SessionData, apperr.Error) {
	// Check if email already exists
	_, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error("failed to check existing user", "error", err)
		return nil, apperr.ErrInternal
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", "error", err)
		return nil, apperr.ErrInternal
	}

	// Create user
	userID := util.GenerateID("usr")
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:            userID,
		Email:         input.Email,
		EmailVerified: false,
		Name:          input.Name,
		Image:         pgutil.NullText(),
	})
	if err != nil {
		s.logger.Error("failed to create user", "error", err)
		return nil, apperr.ErrInternal
	}

	// Create credential account
	accountID := util.GenerateID("acc")
	_, err = s.queries.CreateAccount(ctx, db.CreateAccountParams{
		ID:                accountID,
		UserID:            userID,
		Provider:          "credential",
		ProviderAccountID: input.Email,
		PasswordHash:      pgutil.TextFromString(string(hashedPassword)),
	})
	if err != nil {
		s.logger.Error("failed to create account", "error", err)
		return nil, apperr.ErrInternal
	}

	// Create session
	return s.createSession(ctx, &user, nil, nil)
}

type LoginInput struct {
	Email    string
	Password string
	// Optional: for tracking
	IPAddress *string
	UserAgent *string
}

func (s *service) Login(ctx context.Context, input LoginInput) (*SessionData, apperr.Error) {
	// Get user by email
	user, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		s.logger.Error("failed to get user", "error", err)
		return nil, apperr.ErrInternal
	}

	// Get credential account
	account, err := s.queries.GetAccountByProvider(ctx, db.GetAccountByProviderParams{
		Provider:          "credential",
		ProviderAccountID: input.Email,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		s.logger.Error("failed to get account", "error", err)
		return nil, apperr.ErrInternal
	}

	// Verify password
	if !account.PasswordHash.Valid {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash.String), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Create session
	return s.createSession(ctx, &user, input.IPAddress, input.UserAgent)
}

func (s *service) Logout(ctx context.Context, token string) apperr.Error {
	err := s.queries.DeleteSessionByToken(ctx, token)
	if err != nil {
		s.logger.Error("failed to delete session", "error", err)
		return apperr.ErrInternal
	}
	return nil
}

// createSession creates a new session for a user
func (s *service) createSession(ctx context.Context, user *db.User, ipAddress, userAgent *string) (*SessionData, apperr.Error) {
	sessionID := util.GenerateID("ses")
	token := generateSessionToken()

	session, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:                   sessionID,
		Token:                token,
		UserID:               user.ID,
		ActiveOrganizationID: pgutil.NullText(),
		IpAddress:            pgutil.Text(ipAddress),
		UserAgent:            pgutil.Text(userAgent),
		ExpiresAt:            pgutil.Timestamptz(time.Now().Add(s.sessionExpiresIn)),
	})
	if err != nil {
		s.logger.Error("failed to create session", "error", err)
		return nil, apperr.ErrInternal
	}

	return &SessionData{
		Session: &Session{
			ID:                   session.ID,
			Token:                session.Token,
			UserID:               session.UserID,
			ActiveOrganizationID: pgutil.TextToPtr(session.ActiveOrganizationID),
			IPAddress:            pgutil.TextToPtr(session.IpAddress),
			UserAgent:            pgutil.TextToPtr(session.UserAgent),
			ExpiresAt:            pgutil.TimestamptzToTime(session.ExpiresAt),
			CreatedAt:            pgutil.TimestamptzToTime(session.CreatedAt),
		},
		User: &User{
			ID:            user.ID,
			Email:         user.Email,
			EmailVerified: user.EmailVerified,
			Name:          user.Name,
			Image:         pgutil.TextToPtr(user.Image),
			CreatedAt:     pgutil.TimestamptzToTime(user.CreatedAt),
		},
		ActiveOrganization: nil,
	}, nil
}

// generateSessionToken generates a cryptographically secure session token
func generateSessionToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}
