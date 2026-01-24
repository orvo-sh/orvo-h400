package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

type SetActiveOrganizationInput struct {
	SessionToken   string
	OrganizationID *string
}

func (s *service) SetActiveOrganization(ctx context.Context, input SetActiveOrganizationInput) apperr.Error {
	s.logger.InfoContext(ctx, "SetActiveOrganization: setting active organization", slog.Any("input", input))

	session, err := s.GetSession(ctx, input.SessionToken)
	if err != nil {
		return err
	}

	if input.OrganizationID != nil {
		if _, err := s.postgres.Queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
			OrganizationID: *input.OrganizationID,
			UserID:         session.UserID,
		}); err != nil {
			if pgutil.IsNoRowsError(err) {
				return errs.ErrNotOrganizationMember
			}
			s.logger.ErrorContext(ctx, "SetActiveOrganization: failed to verify organization membership", slog.Any("error", err))
			return errs.ErrInternal
		}
	}

	if _, err := s.postgres.Queries.SetActiveOrganization(ctx, db.SetActiveOrganizationParams{
		ID:                   session.ID,
		ActiveOrganizationID: pgutil.Text(input.OrganizationID),
	}); err != nil {
		s.logger.ErrorContext(ctx, "SetActiveOrganization: failed to set active organization", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
