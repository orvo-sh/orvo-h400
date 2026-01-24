package models

import "time"

type Session struct {
	ID                   string    `json:"id"`
	Token                string    `json:"-"`
	UserID               string    `json:"user_id"`
	ActiveOrganizationID *string   `json:"active_organization_id"`
	IpAddress            *string   `json:"ip_address"`
	UserAgent            *string   `json:"user_agent"`
	ExpiresAt            time.Time `json:"expires_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
