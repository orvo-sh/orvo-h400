package models

import "time"

type OrganizationMemberRole string

const (
	OrganizationMemberRoleOwner  OrganizationMemberRole = "owner"
	OrganizationMemberRoleAdmin  OrganizationMemberRole = "admin"
	OrganizationMemberRoleMember OrganizationMemberRole = "member"
)

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Logo      *string   `json:"logo"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationMember struct {
	ID             string                 `json:"id"`
	OrganizationID string                 `json:"organization_id"`
	UserID         string                 `json:"user_id"`
	Role           OrganizationMemberRole `json:"role"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type OrganizationInvitation struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	InvitedByID    string    `json:"invited_by_id"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
