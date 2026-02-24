package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

type GetSessionDataOutput_Organization struct {
	models.Organization
	MemberRole models.OrganizationMemberRole `json:"member_role"`
}

func (s *service) GetSessionData(ctx context.Context, token string) (*models.Session, *models.User, *GetSessionDataOutput_Organization, apperr.Error) {
	s.logger.InfoContext(ctx, "GetSessionData: getting session", slog.String("token", token))

	row, err := s.postgres.Queries.GetSessionWithUserAndOrganization(ctx, token)
	if err != nil {
		if pgutil.IsNoRowsError(err) {
			return nil, nil, nil, errs.ErrSessionNotFound
		}
		s.logger.ErrorContext(ctx, "GetSessionData: failed to get session", slog.Any("error", err))
		return nil, nil, nil, errs.ErrInternal
	}

	if row.ExpiresAt.Time.Before(time.Now()) {
		return nil, nil, nil, errs.ErrSessionExpired
	}

	session := &models.Session{
		ID:                   row.SessionID,
		Token:                row.Token,
		UserID:               row.UserID,
		ActiveOrganizationID: pgutil.TextToPtr(row.ActiveOrganizationID),
		IpAddress:            pgutil.TextToPtr(row.IpAddress),
		UserAgent:            pgutil.TextToPtr(row.UserAgent),
		ExpiresAt:            row.ExpiresAt.Time,
		CreatedAt:            row.SessionCreatedAt.Time,
		UpdatedAt:            row.SessionUpdatedAt.Time,
	}
	user := &models.User{
		ID:            row.UserID,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		Name:          row.UserName,
		Image:         pgutil.TextToPtr(row.UserImage),
		CreatedAt:     row.UserCreatedAt.Time,
		UpdatedAt:     row.UserUpdatedAt.Time,
	}

	if row.ActiveOrganizationID.Valid && row.OrgID.Valid && row.MemberRole.Valid {
		return session, user, &GetSessionDataOutput_Organization{
			Organization: models.Organization{
				ID:        row.OrgID.String,
				Name:      row.OrgName.String,
				Logo:      pgutil.TextToPtr(row.OrgLogo),
				CreatedAt: row.OrgCreatedAt.Time,
				UpdatedAt: row.OrgUpdatedAt.Time,
			},
			MemberRole: models.OrganizationMemberRole(row.MemberRole.String),
		}, nil
	}

	memberships, err := s.postgres.Queries.GetUserOrganizationMemberships(ctx, row.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetSessionData: failed to fetch user memberships", slog.Any("error", err))
		return nil, nil, nil, errs.ErrInternal
	}
	if len(memberships) == 0 {
		return session, user, nil, nil
	}

	selectedMembership := memberships[0]
	org, err := s.postgres.Queries.GetOrganizationByID(ctx, selectedMembership.OrganizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetSessionData: failed to load selected organization", slog.Any("error", err))
		return nil, nil, nil, errs.ErrInternal
	}

	if _, err := s.postgres.Queries.SetActiveOrganization(ctx, db.SetActiveOrganizationParams{
		ID:                   row.SessionID,
		ActiveOrganizationID: pgutil.Text(&selectedMembership.OrganizationID),
	}); err != nil {
		s.logger.ErrorContext(ctx, "GetSessionData: failed to persist active organization", slog.Any("error", err))
		return nil, nil, nil, errs.ErrInternal
	}

	session.ActiveOrganizationID = &selectedMembership.OrganizationID
	return session, user, &GetSessionDataOutput_Organization{
		Organization: models.Organization{
			ID:        org.ID,
			Name:      org.Name,
			Logo:      pgutil.TextToPtr(org.Logo),
			CreatedAt: org.CreatedAt.Time,
			UpdatedAt: org.UpdatedAt.Time,
		},
		MemberRole: models.OrganizationMemberRole(selectedMembership.Role),
	}, nil
}
