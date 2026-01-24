package organizationservice

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type CreateInvitationInput struct {
	OrganizationID string
	Email          string
	Role           string
	InviterUserID  string
}

// CreateInvitation creates an invitation to join an organization
func (s *service) CreateInvitation(ctx context.Context, input CreateInvitationInput) (*Invitation, apperr.Error) {
	// Verify inviter has permission
	inviter, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: input.OrganizationID,
		UserID:         input.InviterUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotMember
		}
		s.logger.Error("failed to get inviter member", "error", err)
		return nil, apperr.ErrInternal
	}

	if inviter.Role != RoleOwner && inviter.Role != RoleAdmin {
		return nil, ErrNotAuthorized
	}

	// Cannot invite as owner
	if input.Role == RoleOwner {
		return nil, apperr.New(400, "cannot_invite_as_owner")
	}

	// Check for existing pending invitation
	existingInv, err := s.queries.GetPendingInvitationByEmailAndOrg(ctx, db.GetPendingInvitationByEmailAndOrgParams{
		Email:          input.Email,
		OrganizationID: input.OrganizationID,
	})
	if err == nil {
		// Return existing invitation
		return &Invitation{
			ID:             existingInv.ID,
			OrganizationID: existingInv.OrganizationID,
			Email:          existingInv.Email,
			Role:           existingInv.Role,
			InvitedByID:    existingInv.InvitedByID,
			Status:         existingInv.Status,
			ExpiresAt:      pgutil.TimestamptzToTime(existingInv.ExpiresAt),
			CreatedAt:      pgutil.TimestamptzToTime(existingInv.CreatedAt),
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error("failed to check existing invitation", "error", err)
		return nil, apperr.ErrInternal
	}

	// Create invitation
	invID := util.GenerateID("inv")
	expiresAt := time.Now().Add(s.invitationExpiresIn)

	inv, err := s.queries.CreateInvitation(ctx, db.CreateInvitationParams{
		ID:             invID,
		OrganizationID: input.OrganizationID,
		Email:          input.Email,
		Role:           input.Role,
		InvitedByID:    input.InviterUserID,
		ExpiresAt:      pgutil.Timestamptz(expiresAt),
	})
	if err != nil {
		s.logger.Error("failed to create invitation", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Invitation{
		ID:             inv.ID,
		OrganizationID: inv.OrganizationID,
		Email:          inv.Email,
		Role:           inv.Role,
		InvitedByID:    inv.InvitedByID,
		Status:         inv.Status,
		ExpiresAt:      pgutil.TimestamptzToTime(inv.ExpiresAt),
		CreatedAt:      pgutil.TimestamptzToTime(inv.CreatedAt),
	}, nil
}

// AcceptInvitation accepts an invitation and adds the user as a member
func (s *service) AcceptInvitation(ctx context.Context, invitationID, userID string) (*Member, apperr.Error) {
	// Get the invitation
	inv, err := s.queries.GetInvitationByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvitationNotFound
		}
		s.logger.Error("failed to get invitation", "error", err)
		return nil, apperr.ErrInternal
	}

	// Check if invitation is still pending
	if inv.Status != "pending" {
		return nil, apperr.New(400, "invitation_not_pending")
	}

	// Check if invitation has expired
	if pgutil.TimestamptzToTime(inv.ExpiresAt).Before(time.Now()) {
		return nil, ErrInvitationExpired
	}

	// Get the user to verify email matches
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user", "error", err)
		return nil, apperr.ErrInternal
	}

	if user.Email != inv.Email {
		return nil, apperr.New(403, "email_mismatch")
	}

	// Check if already a member
	_, err = s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: inv.OrganizationID,
		UserID:         userID,
	})
	if err == nil {
		// Already a member, just mark invitation as accepted
		_, _ = s.queries.UpdateInvitationStatus(ctx, db.UpdateInvitationStatusParams{
			ID:     invitationID,
			Status: "accepted",
		})
		return nil, ErrAlreadyMember
	}

	// Create member
	memberID := util.GenerateID("mem")
	member, err := s.queries.CreateOrganizationMember(ctx, db.CreateOrganizationMemberParams{
		ID:             memberID,
		OrganizationID: inv.OrganizationID,
		UserID:         userID,
		Role:           inv.Role,
	})
	if err != nil {
		s.logger.Error("failed to create member", "error", err)
		return nil, apperr.ErrInternal
	}

	// Update invitation status
	_, err = s.queries.UpdateInvitationStatus(ctx, db.UpdateInvitationStatusParams{
		ID:     invitationID,
		Status: "accepted",
	})
	if err != nil {
		s.logger.Error("failed to update invitation status", "error", err)
		// Don't fail - member was already created
	}

	return &Member{
		ID:             member.ID,
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           member.Role,
		CreatedAt:      pgutil.TimestamptzToTime(member.CreatedAt),
	}, nil
}

// RejectInvitation rejects an invitation
func (s *service) RejectInvitation(ctx context.Context, invitationID, userID string) apperr.Error {
	inv, err := s.queries.GetInvitationByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvitationNotFound
		}
		s.logger.Error("failed to get invitation", "error", err)
		return apperr.ErrInternal
	}

	// Verify the user's email matches
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user", "error", err)
		return apperr.ErrInternal
	}

	if user.Email != inv.Email {
		return apperr.New(403, "email_mismatch")
	}

	_, err = s.queries.UpdateInvitationStatus(ctx, db.UpdateInvitationStatusParams{
		ID:     invitationID,
		Status: "rejected",
	})
	if err != nil {
		s.logger.Error("failed to reject invitation", "error", err)
		return apperr.ErrInternal
	}

	return nil
}

// CancelInvitation cancels an invitation (by org admin/owner)
func (s *service) CancelInvitation(ctx context.Context, invitationID, actorUserID string) apperr.Error {
	inv, err := s.queries.GetInvitationByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvitationNotFound
		}
		s.logger.Error("failed to get invitation", "error", err)
		return apperr.ErrInternal
	}

	// Verify actor has permission
	actor, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: inv.OrganizationID,
		UserID:         actorUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		s.logger.Error("failed to get actor member", "error", err)
		return apperr.ErrInternal
	}

	if actor.Role != RoleOwner && actor.Role != RoleAdmin {
		return ErrNotAuthorized
	}

	_, err = s.queries.UpdateInvitationStatus(ctx, db.UpdateInvitationStatusParams{
		ID:     invitationID,
		Status: "cancelled",
	})
	if err != nil {
		s.logger.Error("failed to cancel invitation", "error", err)
		return apperr.ErrInternal
	}

	return nil
}

// ListInvitations lists all invitations for an organization
func (s *service) ListInvitations(ctx context.Context, organizationID string) ([]*Invitation, apperr.Error) {
	invitations, err := s.queries.ListInvitationsByOrganizationID(ctx, organizationID)
	if err != nil {
		s.logger.Error("failed to list invitations", "error", err)
		return nil, apperr.ErrInternal
	}

	result := make([]*Invitation, len(invitations))
	for i, inv := range invitations {
		result[i] = &Invitation{
			ID:             inv.ID,
			OrganizationID: inv.OrganizationID,
			Email:          inv.Email,
			Role:           inv.Role,
			InvitedByID:    inv.InvitedByID,
			Status:         inv.Status,
			ExpiresAt:      pgutil.TimestamptzToTime(inv.ExpiresAt),
			CreatedAt:      pgutil.TimestamptzToTime(inv.CreatedAt),
			InviterName:    &inv.InviterName,
			InviterEmail:   &inv.InviterEmail,
		}
	}

	return result, nil
}

// ListUserInvitations lists all pending invitations for a user (by email)
func (s *service) ListUserInvitations(ctx context.Context, email string) ([]*Invitation, apperr.Error) {
	invitations, err := s.queries.ListPendingInvitationsByEmail(ctx, email)
	if err != nil {
		s.logger.Error("failed to list user invitations", "error", err)
		return nil, apperr.ErrInternal
	}

	result := make([]*Invitation, len(invitations))
	for i, inv := range invitations {
		result[i] = &Invitation{
			ID:             inv.ID,
			OrganizationID: inv.OrganizationID,
			Email:          inv.Email,
			Role:           inv.Role,
			InvitedByID:    inv.InvitedByID,
			Status:         inv.Status,
			ExpiresAt:      pgutil.TimestamptzToTime(inv.ExpiresAt),
			CreatedAt:      pgutil.TimestamptzToTime(inv.CreatedAt),
			OrgName:        &inv.OrgName,
			OrgSlug:        &inv.OrgSlug,
			InviterName:    &inv.InviterName,
		}
	}

	return result, nil
}
