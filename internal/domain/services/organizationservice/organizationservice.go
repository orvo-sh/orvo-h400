package organizationservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type Service interface {
	CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*models.Organization, apperr.Error)
}

type service struct {
	postgres *postgres.DB
	logger   *slog.Logger
	config   Config
}

type Config struct {
	MaxOrganizationsPerUser int
}

func New(postgres *postgres.DB, logger *slog.Logger, config Config) Service {
	return &service{
		postgres: postgres,
		logger:   logger,
		config:   config,
	}
}
