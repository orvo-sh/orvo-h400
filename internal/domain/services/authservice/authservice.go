package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	Register(ctx context.Context, input RegisterInput) (*models.Session, apperr.Error)
	Login(ctx context.Context, input LoginInput) (*models.Session, apperr.Error)
	Logout(ctx context.Context, token string) apperr.Error
	GetSession(ctx context.Context, token string) (*models.Session, apperr.Error)
	GetSessionData(ctx context.Context, token string) (*models.Session, *models.User, *GetSessionDataOutput_Organization, apperr.Error)
	SetActiveOrganization(ctx context.Context, input SetActiveOrganizationInput) apperr.Error
}

type service struct {
	postgres *postgres.DB
	logger   *slog.Logger
	config   Config
}

type Config struct {
	SessionExpiresIn time.Duration
	SessionUpdateAge time.Duration
}

func New(postgres *postgres.DB, logger *slog.Logger, config Config) Service {
	return &service{
		postgres: postgres,
		logger:   logger,
		config:   config,
	}
}
