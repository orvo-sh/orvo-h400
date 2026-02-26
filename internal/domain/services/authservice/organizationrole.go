package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

func (s *service) EnsureOrganizationRole(ctx context.Context, userID string, organizationID string, allowedRoles ...models.OrganizationMemberRole) apperr.Error {
	s.logger.DebugContext(ctx, "EnsureOrganizationRole: validating role",
		slog.String("user_id", userID),
		slog.String("organization_id", organizationID),
		slog.Any("allowed_roles", allowedRoles),
	)

	member, err := s.postgres.Queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         userID,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return errs.ErrNotOrganizationMember
		}
		s.logger.ErrorContext(ctx, "EnsureOrganizationRole: query failed", slog.Any("error", err))
		return errs.ErrInternal
	}

	if len(allowedRoles) == 0 {
		return nil
	}

	current := models.OrganizationMemberRole(member.Role)
	for _, role := range allowedRoles {
		if current == role {
			return nil
		}
	}

	return errs.ErrForbidden
}
