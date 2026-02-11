package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	dto "github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
	"github.com/orvo-sh/orvo/pkg/sensitive"
)

type AuthHandler struct {
	authService authservice.Service
	config      NewAuthConfig
}

type NewAuthConfig struct {
	SessionCookieKey       string
	SessionCookieDomain    string
	SessionCookieSecure    bool
	SessionCookieSameSite  http.SameSite
	SessionCookieExpiresIn time.Duration
}

func NewAuthHandler(authService authservice.Service, config NewAuthConfig) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		config:      config,
	}
}

func (h *AuthHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "register",
		Method:      http.MethodPost,
		Path:        "/auth/register",
		Tags:        []string{"auth"},
	}, h.register)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Tags:        []string{"auth"},
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/auth/logout",
		Tags:        []string{"auth"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.logout)

	huma.Register(api, huma.Operation{
		OperationID: "get-session",
		Method:      http.MethodGet,
		Path:        "/auth/session",
		Tags:        []string{"auth"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getSession)

	huma.Register(api, huma.Operation{
		OperationID: "set-active-organization",
		Method:      http.MethodPost,
		Path:        "/auth/session/active-organization",
		Tags:        []string{"auth"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.SetActiveOrganization)
}

func (h *AuthHandler) register(ctx context.Context, input *dto.RegisterInput) (*dto.SessionOutput, error) {
	if session, err := h.authService.Register(ctx, authservice.RegisterInput{
		Email:    input.Body.Email,
		Password: sensitive.New(input.Body.Password),
		Name:     input.Body.Name,
	}); err != nil {
		return nil, err
	} else {
		return &dto.SessionOutput{
			SetCookie: []http.Cookie{
				{
					Name:     h.config.SessionCookieKey,
					Path:     "/",
					Value:    session.Token,
					Domain:   h.config.SessionCookieDomain,
					Secure:   h.config.SessionCookieSecure,
					SameSite: h.config.SessionCookieSameSite,
					MaxAge:   int(h.config.SessionCookieExpiresIn.Seconds()),
					HttpOnly: true,
				},
			},
		}, nil
	}
}

func (h *AuthHandler) login(ctx context.Context, input *dto.LoginInput) (*dto.SessionOutput, error) {
	if session, err := h.authService.Login(ctx, authservice.LoginInput{
		Email:    input.Body.Email,
		Password: sensitive.New(input.Body.Password),
	}); err != nil {
		return nil, err
	} else {
		return &dto.SessionOutput{
			SetCookie: []http.Cookie{
				{
					Name:     h.config.SessionCookieKey,
					Path:     "/",
					Value:    session.Token,
					Domain:   h.config.SessionCookieDomain,
					Secure:   h.config.SessionCookieSecure,
					SameSite: h.config.SessionCookieSameSite,
					MaxAge:   int(h.config.SessionCookieExpiresIn.Seconds()),
					HttpOnly: true,
				},
			},
		}, nil
	}
}

func (h *AuthHandler) logout(ctx context.Context, input *dto.Empty) (*dto.Empty, error) {
	session := authmiddleware.GetSessionFromContext(ctx)
	if err := h.authService.Logout(ctx, session.Token); err != nil {
		return nil, err
	} else {
		return nil, nil
	}
}

func (h *AuthHandler) getSession(ctx context.Context, input *dto.Empty) (*dto.GetSessionOutput, error) {
	token := authmiddleware.GetSessionFromContext(ctx).Token
	if session, user, activeOrganization, err := h.authService.GetSessionData(ctx, token); err != nil {
		return nil, err
	} else {
		return &dto.GetSessionOutput{
			Body: struct {
				Session            *models.Session                                `json:"session"`
				User               *models.User                                   `json:"user"`
				ActiveOrganization *authservice.GetSessionDataOutput_Organization `json:"active_organization"`
			}{
				Session:            session,
				User:               user,
				ActiveOrganization: activeOrganization,
			},
		}, nil
	}
}

func (h *AuthHandler) SetActiveOrganization(ctx context.Context, input *dto.SetActiveOrganizationInput) (*dto.Empty, error) {
	session := authmiddleware.GetSessionFromContext(ctx)

	err := h.authService.SetActiveOrganization(ctx, authservice.SetActiveOrganizationInput{
		SessionToken:   session.Token,
		OrganizationID: input.Body.OrganizationID,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
