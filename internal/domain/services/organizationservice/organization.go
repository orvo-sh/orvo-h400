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

var (
	ErrOrgNotFound        = apperr.New(404, "organization_not_found")
	ErrSlugAlreadyUsed    = apperr.New(409, "slug_already_used")
	ErrNotAuthorized      = apperr.New(403, "not_authorized")
	ErrNotMember          = apperr.New(403, "not_organization_member")
	ErrCannotRemoveOwner  = apperr.New(400, "cannot_remove_owner")
	ErrMemberNotFound     = apperr.New(404, "member_not_found")
	ErrInvitationNotFound = apperr.New(404, "invitation_not_found")
	ErrInvitationExpired  = apperr.New(400, "invitation_expired")
	ErrAlreadyMember      = apperr.New(409, "already_member")
)

type CreateInput struct {
	Name     string
	Slug     string
	Logo     *string
	Metadata []byte
	// The user creating the organization (will become owner)
	CreatorUserID string
}

// Create creates a new organization and adds the creator as owner
func (s *service) Create(ctx context.Context, input CreateInput) (*Organization, apperr.Error) {
	// Check if slug is already taken
	exists, err := s.queries.CheckSlugExists(ctx, input.Slug)
	if err != nil {
		s.logger.Error("failed to check slug", "error", err)
		return nil, apperr.ErrInternal
	}
	if exists {
		return nil, ErrSlugAlreadyUsed
	}

	// Create organization
	orgID := util.GenerateID("org")
	org, err := s.queries.CreateOrganization(ctx, db.CreateOrganizationParams{
		ID:       orgID,
		Name:     input.Name,
		Slug:     input.Slug,
		Logo:     pgutil.Text(input.Logo),
		Metadata: input.Metadata,
	})
	if err != nil {
		s.logger.Error("failed to create organization", "error", err)
		return nil, apperr.ErrInternal
	}

	// Add creator as owner
	memberID := util.GenerateID("mem")
	_, err = s.queries.CreateOrganizationMember(ctx, db.CreateOrganizationMemberParams{
		ID:             memberID,
		OrganizationID: orgID,
		UserID:         input.CreatorUserID,
		Role:           RoleOwner,
	})
	if err != nil {
		s.logger.Error("failed to add owner to organization", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Organization{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Logo:      pgutil.TextToPtr(org.Logo),
		Metadata:  org.Metadata,
		CreatedAt: pgutil.TimestamptzToTime(org.CreatedAt),
	}, nil
}

// GetByID retrieves an organization by ID
func (s *service) GetByID(ctx context.Context, id string) (*Organization, apperr.Error) {
	org, err := s.queries.GetOrganizationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrgNotFound
		}
		s.logger.Error("failed to get organization", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Organization{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Logo:      pgutil.TextToPtr(org.Logo),
		Metadata:  org.Metadata,
		CreatedAt: pgutil.TimestamptzToTime(org.CreatedAt),
	}, nil
}

// GetBySlug retrieves an organization by slug
func (s *service) GetBySlug(ctx context.Context, slug string) (*Organization, apperr.Error) {
	org, err := s.queries.GetOrganizationBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrgNotFound
		}
		s.logger.Error("failed to get organization", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Organization{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Logo:      pgutil.TextToPtr(org.Logo),
		Metadata:  org.Metadata,
		CreatedAt: pgutil.TimestamptzToTime(org.CreatedAt),
	}, nil
}

type UpdateInput struct {
	OrganizationID string
	ActorUserID    string // User performing the update (for authorization)
	Name           *string
	Slug           *string
	Logo           *string
	Metadata       []byte
}

// Update updates an organization (requires admin or owner role)
func (s *service) Update(ctx context.Context, input UpdateInput) (*Organization, apperr.Error) {
	// Check if user has permission
	member, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: input.OrganizationID,
		UserID:         input.ActorUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotMember
		}
		s.logger.Error("failed to get member", "error", err)
		return nil, apperr.ErrInternal
	}

	// Only owner and admin can update
	if member.Role != RoleOwner && member.Role != RoleAdmin {
		return nil, ErrNotAuthorized
	}

	// If slug is being updated, check if new slug is available
	if input.Slug != nil {
		exists, err := s.queries.CheckSlugExists(ctx, *input.Slug)
		if err != nil {
			s.logger.Error("failed to check slug", "error", err)
			return nil, apperr.ErrInternal
		}
		if exists {
			// Check if it's the same org
			existingOrg, _ := s.queries.GetOrganizationBySlug(ctx, *input.Slug)
			if existingOrg.ID != input.OrganizationID {
				return nil, ErrSlugAlreadyUsed
			}
		}
	}

	org, err := s.queries.UpdateOrganization(ctx, db.UpdateOrganizationParams{
		ID:       input.OrganizationID,
		Name:     pgutil.Text(input.Name),
		Slug:     pgutil.Text(input.Slug),
		Logo:     pgutil.Text(input.Logo),
		Metadata: input.Metadata,
	})
	if err != nil {
		s.logger.Error("failed to update organization", "error", err)
		return nil, apperr.ErrInternal
	}

	return &Organization{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		Logo:      pgutil.TextToPtr(org.Logo),
		Metadata:  org.Metadata,
		CreatedAt: pgutil.TimestamptzToTime(org.CreatedAt),
	}, nil
}

// Delete deletes an organization (owner only)
func (s *service) Delete(ctx context.Context, id string, userID string) apperr.Error {
	// Check if user is owner
	member, err := s.queries.GetMemberByOrgAndUser(ctx, db.GetMemberByOrgAndUserParams{
		OrganizationID: id,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		s.logger.Error("failed to get member", "error", err)
		return apperr.ErrInternal
	}

	if member.Role != RoleOwner {
		return ErrNotAuthorized
	}

	// Delete organization (cascades to members and invitations)
	err = s.queries.DeleteOrganization(ctx, id)
	if err != nil {
		s.logger.Error("failed to delete organization", "error", err)
		return apperr.ErrInternal
	}

	return nil
}

// CheckSlugExists checks if a slug is already in use
func (s *service) CheckSlugExists(ctx context.Context, slug string) (bool, apperr.Error) {
	exists, err := s.queries.CheckSlugExists(ctx, slug)
	if err != nil {
		s.logger.Error("failed to check slug", "error", err)
		return false, apperr.ErrInternal
	}
	return exists, nil
}

// ListByUserID returns all organizations a user is a member of
func (s *service) ListByUserID(ctx context.Context, userID string) ([]*Organization, apperr.Error) {
	orgs, err := s.queries.ListOrganizationsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list organizations", "error", err)
		return nil, apperr.ErrInternal
	}

	result := make([]*Organization, len(orgs))
	for i, org := range orgs {
		result[i] = &Organization{
			ID:        org.ID,
			Name:      org.Name,
			Slug:      org.Slug,
			Logo:      pgutil.TextToPtr(org.Logo),
			Metadata:  org.Metadata,
			CreatedAt: pgutil.TimestamptzToTime(org.CreatedAt),
		}
	}

	return result, nil
}
