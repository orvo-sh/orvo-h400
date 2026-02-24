package authservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

func (s *service) EnsureOrganizationMember(ctx context.Context, userID string, organizationID string) apperr.Error {
	s.logger.DebugContext(ctx, "EnsureOrganizationMember: validating membership",
		slog.String("user_id", userID),
		slog.String("organization_id", organizationID),
	)

	_, err := s.postgres.Queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         userID,
	})
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return errs.ErrNotOrganizationMember
		}
		s.logger.ErrorContext(ctx, "EnsureOrganizationMember: query failed", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}
