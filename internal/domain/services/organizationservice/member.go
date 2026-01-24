package organizationservice

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type AddMemberInput struct {
	OrganizationID string
	UserID         string
	Role           string
	ActorUserID    string // User performing the action (for authorization)
}

// AddMember adds a user to an organization
func (s *service) AddMember(ctx context.Context, input AddMemberInput) (*Member, apperr.Error) {
	// Verify actor has permission (owner or admin)
	actorMember, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: input.OrganizationID,
		UserID:         input.ActorUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotMember
		}
		s.logger.Error("failed to get actor member", "error", err)
		return nil, apperr.ErrInternal
	}

	if actorMember.Role != RoleOwner && actorMember.Role != RoleAdmin {
		return nil, ErrNotAuthorized
	}

	// Check if user is already a member
	_, err = s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
	})
	if err == nil {
		return nil, ErrAlreadyMember
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error("failed to check existing member", "error", err)
		return nil, apperr.ErrInternal
	}

	// Cannot add owner role (only one owner per org)
	if input.Role == RoleOwner {
		return nil, apperr.New(400, "cannot_add_owner")
	}

	// Create member
	memberID := util.GenerateID("mem")
	member, err := s.queries.CreateOrganizationMember(ctx, db.CreateOrganizationMemberParams{
		ID:             memberID,
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
		Role:           input.Role,
	})
	if err != nil {
		s.logger.Error("failed to create member", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Member{
		ID:             member.ID,
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           member.Role,
		CreatedAt:      pgutil.TimestamptzToTime(member.CreatedAt),
	}, nil
}

// RemoveMember removes a member from an organization
func (s *service) RemoveMember(ctx context.Context, organizationID, memberID string, actorUserID string) apperr.Error {
	// Get the member being removed
	member, err := s.queries.GetMemberByID(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		s.logger.Error("failed to get member", "error", err)
		return apperr.ErrInternal
	}

	// Cannot remove owner
	if member.Role == RoleOwner {
		return ErrCannotRemoveOwner
	}

	// Verify actor has permission
	actorMember, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         actorUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		s.logger.Error("failed to get actor member", "error", err)
		return apperr.ErrInternal
	}

	if actorMember.Role != RoleOwner && actorMember.Role != RoleAdmin {
		return ErrNotAuthorized
	}

	// Delete member
	err = s.queries.DeleteMember(ctx, memberID)
	if err != nil {
		s.logger.Error("failed to delete member", "error", err)
		return apperr.ErrInternal
	}

	return nil
}

type UpdateMemberRoleInput struct {
	OrganizationID string
	MemberID       string
	NewRole        string
	ActorUserID    string
}

// UpdateMemberRole updates a member's role
func (s *service) UpdateMemberRole(ctx context.Context, input UpdateMemberRoleInput) (*Member, apperr.Error) {
	// Get the member being updated
	member, err := s.queries.GetMemberByID(ctx, input.MemberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		s.logger.Error("failed to get member", "error", err)
		return nil, apperr.ErrInternal
	}

	// Cannot change owner's role
	if member.Role == RoleOwner {
		return nil, apperr.New(400, "cannot_change_owner_role")
	}

	// Cannot assign owner role
	if input.NewRole == RoleOwner {
		return nil, apperr.New(400, "cannot_assign_owner_role")
	}

	// Verify actor is owner or admin
	actorMember, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: input.OrganizationID,
		UserID:         input.ActorUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotMember
		}
		s.logger.Error("failed to get actor member", "error", err)
		return nil, apperr.ErrInternal
	}

	if actorMember.Role != RoleOwner && actorMember.Role != RoleAdmin {
		return nil, ErrNotAuthorized
	}

	// Update role
	updated, err := s.queries.UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
		ID:   input.MemberID,
		Role: input.NewRole,
	})
	if err != nil {
		s.logger.Error("failed to update member role", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Member{
		ID:             updated.ID,
		OrganizationID: updated.OrganizationID,
		UserID:         updated.UserID,
		Role:           updated.Role,
		CreatedAt:      pgutil.TimestamptzToTime(updated.CreatedAt),
	}, nil
}

// ListMembers lists all members of an organization
func (s *service) ListMembers(ctx context.Context, organizationID string, limit, offset int32) ([]*Member, apperr.Error) {
	members, err := s.queries.ListMembersByOrganizationID(ctx, db.ListMembersByOrganizationIDParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		s.logger.Error("failed to list members", "error", err)
		return nil, apperr.ErrInternal
	}

	result := make([]*Member, len(members))
	for i, m := range members {
		result[i] = &Member{
			ID:             m.ID,
			OrganizationID: m.OrganizationID,
			UserID:         m.UserID,
			Role:           m.Role,
			CreatedAt:      pgutil.TimestamptzToTime(m.CreatedAt),
			Email:          &m.Email,
			Name:           &m.Name,
			Image:          pgutil.TextToPtr(m.Image),
		}
	}

	return result, nil
}

// GetMember gets a specific member by organization and user ID
func (s *service) GetMember(ctx context.Context, organizationID, userID string) (*Member, apperr.Error) {
	member, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMemberNotFound
		}
		s.logger.Error("failed to get member", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Member{
		ID:             member.ID,
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           member.Role,
		CreatedAt:      pgutil.TimestamptzToTime(member.CreatedAt),
	}, nil
}

// Leave removes the current user from an organization
func (s *service) Leave(ctx context.Context, organizationID, userID string) apperr.Error {
	member, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		s.logger.Error("failed to get member", "error", err)
		return apperr.ErrInternal
	}

	// Owner cannot leave (must transfer ownership first or delete org)
	if member.Role == RoleOwner {
		return apperr.New(400, "owner_cannot_leave")
	}

	err = s.queries.DeleteMemberByOrgAndUser(ctx, db.DeleteMemberByOrgAndUserParams{
		OrganizationID: organizationID,
		UserID:         userID,
	})
	if err != nil {
		s.logger.Error("failed to leave organization", "error", err)
		return apperr.ErrInternal
	}

	return nil
}
