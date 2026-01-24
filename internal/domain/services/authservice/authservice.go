package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

// SessionData represents session information including user and optional active organization
// This mirrors better-auth's session concept where a session can have an active organization
type SessionData struct {
	Session            *Session
	User               *User
	ActiveOrganization *ActiveOrganization
}

type Session struct {
	ID                   string
	Token                string
	UserID               string
	ActiveOrganizationID *string
	IPAddress            *string
	UserAgent            *string
	ExpiresAt            time.Time
	CreatedAt            time.Time
}

type User struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	Image         *string
	CreatedAt     time.Time
}

// ActiveOrganization represents the currently active organization for a session
// Similar to better-auth's activeOrganization concept
type ActiveOrganization struct {
	ID         string
	Name       string
	Slug       string
	Logo       *string
	MemberRole string // The user's role in this organization
}

type Service interface {
	// Registration & Authentication
	Register(ctx context.Context, input RegisterInput) (*SessionData, apperr.Error)
	Login(ctx context.Context, input LoginInput) (*SessionData, apperr.Error)
	Logout(ctx context.Context, token string) apperr.Error

	// Session Management
	GetSession(ctx context.Context, token string) (*SessionData, apperr.Error)
	RefreshSession(ctx context.Context, token string) (*SessionData, apperr.Error)
	ListSessions(ctx context.Context, userID string) ([]*Session, apperr.Error)
	RevokeSession(ctx context.Context, sessionID string) apperr.Error
	RevokeAllSessions(ctx context.Context, userID string) apperr.Error

	// Active Organization (inspired by better-auth)
	SetActiveOrganization(ctx context.Context, sessionToken string, organizationID *string) (*SessionData, apperr.Error)

	// User Management
	GetUser(ctx context.Context, userID string) (*User, apperr.Error)
	UpdateUser(ctx context.Context, input UpdateUserInput) (*User, apperr.Error)
}

// Querier interface for database operations (works with both pool and transactions)
type Querier interface {
	db.Querier
}

type service struct {
	queries *db.Queries
	logger  *slog.Logger

	// Session configuration (inspired by better-auth)
	sessionExpiresIn time.Duration // default: 7 days
	sessionUpdateAge time.Duration // when to refresh session: 1 day
}

type Config struct {
	SessionExpiresIn time.Duration
	SessionUpdateAge time.Duration
}

func DefaultConfig() Config {
	return Config{
		SessionExpiresIn: 7 * 24 * time.Hour, // 7 days
		SessionUpdateAge: 24 * time.Hour,     // 1 day
	}
}

func New(logger *slog.Logger, queries *db.Queries, cfg ...Config) Service {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	return &service{
		queries:          queries,
		logger:           logger,
		sessionExpiresIn: config.SessionExpiresIn,
		sessionUpdateAge: config.SessionUpdateAge,
	}
}
