package authservice

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/background"
)

type Service interface {
	Register(ctx context.Context, input RegisterInput) (*models.Session, apperr.Error)
	Login(ctx context.Context, input LoginInput) (*models.Session, apperr.Error)
	Logout(ctx context.Context, token string) apperr.Error
	GetSession(ctx context.Context, token string) (*models.Session, apperr.Error)
	GetSessionData(ctx context.Context, token string) (*models.Session, *models.User, *GetSessionDataOutput_Organization, apperr.Error)
	SetActiveOrganization(ctx context.Context, input SetActiveOrganizationInput) apperr.Error
	EnsureOrganizationMember(ctx context.Context, userID string, organizationID string) apperr.Error
	EnsureOrganizationRole(ctx context.Context, userID string, organizationID string, allowedRoles ...models.OrganizationMemberRole) apperr.Error

	CreateApiKey(ctx context.Context, input CreateApiKeyInput) (*models.ApiKey, *string, apperr.Error)
	ListApiKeys(ctx context.Context, organizationID string) ([]models.ApiKey, apperr.Error)
	ResolveApiKey(ctx context.Context, rawKey string) (*string, apperr.Error)
	RevokeApiKey(ctx context.Context, input RevokeApiKeyInput) apperr.Error
}

type apiKeyCacheEntry struct {
	organizationID string
	expiresAt      time.Time
}

type service struct {
	postgres *postgres.DB
	logger   *slog.Logger

	backgroundManager *background.Manager

	apiKeyResolverMu    sync.RWMutex
	apiKeyResolverCache map[string]apiKeyCacheEntry
	config              Config
}

type Config struct {
	SessionExpiresIn       time.Duration
	SessionUpdateAge       time.Duration
	ApiKeyCacheResolverTTL time.Duration
}

func New(postgres *postgres.DB, logger *slog.Logger, backgroundManager *background.Manager, config Config) Service {
	return &service{
		postgres:            postgres,
		logger:              logger,
		backgroundManager:   backgroundManager,
		config:              config,
		apiKeyResolverCache: make(map[string]apiKeyCacheEntry),
	}
}
