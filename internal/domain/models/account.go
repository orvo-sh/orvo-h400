package models

import (
	"time"

	"github.com/orvo-sh/orvo/pkg/sensitive"
)

type AccountProvider string

const (
	AccountProviderEmail  AccountProvider = "email"
	AccountProviderGoogle AccountProvider = "google"
)

type Account struct {
	ID                string                       `json:"id"`
	UserID            string                       `json:"user_id"`
	Provider          AccountProvider              `json:"provider"`
	ProviderAccountID string                       `json:"provider_account_id"`
	PasswordHash      sensitive.Sensitive[*string] `json:"-"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
}
