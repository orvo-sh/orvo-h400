package dto

import (
	"net/http"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
)

type RegisterInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Password string `json:"password" minLength:"8" maxLength:"72"`
		Name     string `json:"name" minLength:"1" maxLength:"100"`
	}
}

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email" `
		Password string `json:"password" minLength:"8" maxLength:"72"`
	}
}

type SessionOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
}

type GetSessionOutput struct {
	Body struct {
		Session            *models.Session                                `json:"session"`
		User               *models.User                                   `json:"user"`
		ActiveOrganization *authservice.GetSessionDataOutput_Organization `json:"active_organization"`
	}
}

type SetActiveOrganizationInput struct {
	Body struct {
		OrganizationID *string `json:"organization_id"`
	}
}
