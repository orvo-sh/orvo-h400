package dto

import (
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

type CreateApiKeyInput struct {
	Body struct {
		Name      string         `json:"name" minLength:"1" maxLength:"255"`
		ExpiresIn *time.Duration `json:"expires_in,omitempty"`
	}
	OrganizationID string `path:"organization_id"`
}

type CreateApiKeyOutput struct {
	Body struct {
		ApiKey models.ApiKey `json:"api_key"`
		Key    string        `json:"key"`
	}
}

type ListApiKeysInput struct {
	OrganizationID string `path:"organization_id"`
}

type ListApiKeysOutput struct {
	Body struct {
		ApiKeys []models.ApiKey `json:"api_keys"`
	}
}

type RevokeApiKeyInput struct {
	OrganizationID string `path:"organization_id"`
	KeyID          string `path:"key_id"`
}

type ApiKeyOutput struct {
	Body struct {
		ID        string     `json:"id"`
		Name      string     `json:"name"`
		CreatedAt time.Time  `json:"created_at"`
		RevokedAt *time.Time `json:"revoked_at"`
	}
}
