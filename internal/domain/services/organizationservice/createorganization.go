package organizationservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type CreateOrganizationInput struct {
	Name          string
	Logo          *string
	CreatorUserID string
}

func (s *service) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*models.Organization, apperr.Error) {
	s.logger.InfoContext(ctx, "CreateOrganization: creating organization", slog.Any("input", input))

	var organization models.Organization
	err := s.postgres.WithTx(ctx, func(q *postgres.Queries) error {
		dbOrganization, err := s.postgres.Queries.CreateOrganization(ctx, db.CreateOrganizationParams{
			ID:   util.GenerateID("org"),
			Name: input.Name,
			Logo: pgutil.Text(input.Logo),
		})
		if err != nil {
			s.logger.Error("failed to create organization", "error", err)
			return errs.ErrInternal
		}
		if _, err = s.postgres.Queries.CreateOrganizationMember(ctx, db.CreateOrganizationMemberParams{
			ID:             util.GenerateID("mem"),
			OrganizationID: dbOrganization.ID,
			UserID:         input.CreatorUserID,
			Role:           string(models.OrganizationMemberRoleOwner),
		}); err != nil {
			s.logger.Error("CreateOrganization: failed to create organization member", slog.Any("error", err))
			return errs.ErrInternal
		}

		organization = models.Organization{
			ID:        dbOrganization.ID,
			Name:      dbOrganization.Name,
			Logo:      pgutil.TextToPtr(dbOrganization.Logo),
			CreatedAt: dbOrganization.CreatedAt.Time,
			UpdatedAt: dbOrganization.UpdatedAt.Time,
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

	return &organization, nil
}
