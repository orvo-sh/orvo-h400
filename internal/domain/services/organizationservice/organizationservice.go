package organizationservice

import (
	"context"
	"log/slog"
	"time"

	"github.com/orvo-sh/orvo/internal/infra/postgres-db/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

// Organization represents an organization
type Organization struct {
	ID        string
	Name      string
	Slug      string
	Logo      *string
	Metadata  []byte
	CreatedAt time.Time
}

// Member represents a member of an organization
type Member struct {
	ID             string
	OrganizationID string
	UserID         string
	Role           string // 'owner', 'admin', 'member'
	CreatedAt      time.Time
	// User info (when joined)
	Email *string
	Name  *string
	Image *string
}

// Invitation represents an organization invitation
type Invitation struct {
	ID             string
	OrganizationID string
	Email          string
	Role           string
	InvitedByID    string
	Status         string // 'pending', 'accepted', 'rejected', 'cancelled'
	ExpiresAt      time.Time
	CreatedAt      time.Time
	// Extra info
	InviterName  *string
	InviterEmail *string
	OrgName      *string
	OrgSlug      *string
}

// MemberRole constants (inspired by better-auth)
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type Service interface {
	// Organization CRUD
	Create(ctx context.Context, input CreateInput) (*Organization, apperr.Error)
	GetByID(ctx context.Context, id string) (*Organization, apperr.Error)
	GetBySlug(ctx context.Context, slug string) (*Organization, apperr.Error)
	Update(ctx context.Context, input UpdateInput) (*Organization, apperr.Error)
	Delete(ctx context.Context, id string, userID string) apperr.Error
	CheckSlugExists(ctx context.Context, slug string) (bool, apperr.Error)

	// User's organizations
	ListByUserID(ctx context.Context, userID string) ([]*Organization, apperr.Error)

	// Member management (inspired by better-auth)
	AddMember(ctx context.Context, input AddMemberInput) (*Member, apperr.Error)
	RemoveMember(ctx context.Context, organizationID, memberID string, actorUserID string) apperr.Error
	UpdateMemberRole(ctx context.Context, input UpdateMemberRoleInput) (*Member, apperr.Error)
	ListMembers(ctx context.Context, organizationID string, limit, offset int32) ([]*Member, apperr.Error)
	GetMember(ctx context.Context, organizationID, userID string) (*Member, apperr.Error)
	Leave(ctx context.Context, organizationID, userID string) apperr.Error

	// Invitations (inspired by better-auth)
	CreateInvitation(ctx context.Context, input CreateInvitationInput) (*Invitation, apperr.Error)
	AcceptInvitation(ctx context.Context, invitationID, userID string) (*Member, apperr.Error)
	RejectInvitation(ctx context.Context, invitationID, userID string) apperr.Error
	CancelInvitation(ctx context.Context, invitationID, actorUserID string) apperr.Error
	ListInvitations(ctx context.Context, organizationID string) ([]*Invitation, apperr.Error)
	ListUserInvitations(ctx context.Context, email string) ([]*Invitation, apperr.Error)
}

type service struct {
	queries *db.Queries
	logger  *slog.Logger

	// Configuration
	invitationExpiresIn time.Duration
}

type Config struct {
	InvitationExpiresIn time.Duration
}

func DefaultConfig() Config {
	return Config{
		InvitationExpiresIn: 7 * 24 * time.Hour, // 7 days
	}
}

func New(logger *slog.Logger, queries *db.Queries, cfg ...Config) Service {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	return &service{
		queries:             queries,
		logger:              logger,
		invitationExpiresIn: config.InvitationExpiresIn,
	}
}
