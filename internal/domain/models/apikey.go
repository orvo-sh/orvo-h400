package models

import "time"

type ApiKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	KeyHash        string     `json:"-"`
	Name           string     `json:"name"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
}
