package authservice

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
)

// GetSession retrieves a session by token, including user and active organization info
// This is inspired by better-auth's getSession which returns session + user + activeOrg
func (s *service) GetSession(ctx context.Context, token string) (*SessionData, apperr.Error) {
	// Get session with user and organization data in a single query
	row, err := s.queries.GetSessionWithUserAndOrg(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		s.logger.Error("failed to get session", "error", err)
		return nil, apperr.ErrInternal
	}

	// Check if session is expired
	if pgutil.TimestamptzToTime(row.ExpiresAt).Before(time.Now()) {
		return nil, ErrSessionExpired
	}

	// Build session data
	sessionData := &SessionData{
		Session: &Session{
			ID:                   row.SessionID,
			Token:                row.Token,
			UserID:               row.UserID,
			ActiveOrganizationID: pgutil.TextToPtr(row.ActiveOrganizationID),
			IPAddress:            pgutil.TextToPtr(row.IpAddress),
			UserAgent:            pgutil.TextToPtr(row.UserAgent),
			ExpiresAt:            pgutil.TimestamptzToTime(row.ExpiresAt),
			CreatedAt:            pgutil.TimestamptzToTime(row.SessionCreatedAt),
		},
		User: &User{
			ID:            row.UserID,
			Email:         row.Email,
			EmailVerified: row.EmailVerified,
			Name:          row.UserName,
			Image:         pgutil.TextToPtr(row.UserImage),
			CreatedAt:     pgutil.TimestamptzToTime(row.UserCreatedAt),
		},
	}

	// Add active organization if present
	if row.OrgID.Valid {
		sessionData.ActiveOrganization = &ActiveOrganization{
			ID:         row.OrgID.String,
			Name:       row.OrgName.String,
			Slug:       row.OrgSlug.String,
			Logo:       pgutil.TextToPtr(row.OrgLogo),
			MemberRole: pgutil.TextToString(row.MemberRole),
		}
	}

	return sessionData, nil
}

// RefreshSession extends the session expiration if the updateAge threshold is reached
// Inspired by better-auth's session refresh behavior
func (s *service) RefreshSession(ctx context.Context, token string) (*SessionData, apperr.Error) {
	sessionData, err := s.GetSession(ctx, token)
	if err != nil {
		return nil, err
	}

	// Check if session needs refresh (if createdAt + updateAge < now)
	sessionAge := time.Since(sessionData.Session.CreatedAt)
	if sessionAge >= s.sessionUpdateAge {
		// Refresh the session expiry
		newExpiry := time.Now().Add(s.sessionExpiresIn)
		_, dbErr := s.queries.UpdateSessionExpiry(ctx, db.UpdateSessionExpiryParams{
			ID:        sessionData.Session.ID,
			ExpiresAt: pgutil.Timestamptz(newExpiry),
		})
		if dbErr != nil {
			s.logger.Error("failed to refresh session", "error", dbErr)
			return nil, apperr.ErrInternal
		}
		sessionData.Session.ExpiresAt = newExpiry
	}

	return sessionData, nil
}

// ListSessions returns all active sessions for a user
func (s *service) ListSessions(ctx context.Context, userID string) ([]*Session, apperr.Error) {
	sessions, err := s.queries.GetSessionsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list sessions", "error", err)
		return nil, apperr.ErrInternal
	}

	result := make([]*Session, len(sessions))
	for i, sess := range sessions {
		result[i] = &Session{
			ID:                   sess.ID,
			Token:                sess.Token,
			UserID:               sess.UserID,
			ActiveOrganizationID: pgutil.TextToPtr(sess.ActiveOrganizationID),
			IPAddress:            pgutil.TextToPtr(sess.IpAddress),
			UserAgent:            pgutil.TextToPtr(sess.UserAgent),
			ExpiresAt:            pgutil.TimestamptzToTime(sess.ExpiresAt),
			CreatedAt:            pgutil.TimestamptzToTime(sess.CreatedAt),
		}
	}

	return result, nil
}

// RevokeSession deletes a specific session
func (s *service) RevokeSession(ctx context.Context, sessionID string) apperr.Error {
	err := s.queries.DeleteSession(ctx, sessionID)
	if err != nil {
		s.logger.Error("failed to revoke session", "error", err)
		return apperr.ErrInternal
	}
	return nil
}

// RevokeAllSessions deletes all sessions for a user
func (s *service) RevokeAllSessions(ctx context.Context, userID string) apperr.Error {
	err := s.queries.DeleteSessionsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to revoke all sessions", "error", err)
		return apperr.ErrInternal
	}
	return nil
}

// SetActiveOrganization sets the active organization for a session
// Inspired by better-auth's setActive function
// Pass nil to unset the active organization
func (s *service) SetActiveOrganization(ctx context.Context, sessionToken string, organizationID *string) (*SessionData, apperr.Error) {
	// First get the current session to verify it exists
	currentSession, err := s.GetSession(ctx, sessionToken)
	if err != nil {
		return nil, err
	}

	// If organizationID is provided, verify the user is a member of that organization
	if organizationID != nil {
		_, dbErr := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
			OrganizationID: *organizationID,
			UserID:         currentSession.User.ID,
		})
		if dbErr != nil {
			if errors.Is(dbErr, pgx.ErrNoRows) {
				return nil, apperr.New(403, "not_organization_member")
			}
			s.logger.Error("failed to verify organization membership", "error", dbErr)
			return nil, apperr.ErrInternal
		}
	}

	// Update the session's active organization
	_, dbErr := s.queries.SetActiveOrganization(ctx, db.SetActiveOrganizationParams{
		ID:                   currentSession.Session.ID,
		ActiveOrganizationID: pgutil.Text(organizationID),
	})
	if dbErr != nil {
		s.logger.Error("failed to set active organization", "error", dbErr)
		return nil, apperr.ErrInternal
	}

	// Return updated session data
	return s.GetSession(ctx, sessionToken)
}
