package authservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
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

	return &models.Session{
			ID:                   row.SessionID,
			Token:                row.Token,
			UserID:               row.UserID,
			ActiveOrganizationID: pgutil.TextToPtr(row.ActiveOrganizationID),
			IpAddress:            pgutil.TextToPtr(row.IpAddress),
			UserAgent:            pgutil.TextToPtr(row.UserAgent),
			ExpiresAt:            row.ExpiresAt.Time,
			CreatedAt:            row.SessionCreatedAt.Time,
			UpdatedAt:            row.SessionUpdatedAt.Time,
		}, &models.User{
			ID:            row.UserID,
			Email:         row.Email,
			EmailVerified: row.EmailVerified,
			Name:          row.UserName,
			Image:         pgutil.TextToPtr(row.UserImage),
			CreatedAt:     row.UserCreatedAt.Time,
			UpdatedAt:     row.UserUpdatedAt.Time,
		}, &GetSessionDataOutput_Organization{
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
