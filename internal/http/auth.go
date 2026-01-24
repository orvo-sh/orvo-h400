package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/http/helper"
	"github.com/orvo-sh/orvo/internal/http/middleware/bodyparser"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

type authHttpHandler struct {
	authService authservice.Service
}

// Request types
type (
	registerRequest struct {
		Body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		} `in:"body=json"`
	}

	loginRequest struct {
		Body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		} `in:"body=json"`
	}

	setActiveOrganizationRequest struct {
		Body struct {
			OrganizationID *string `json:"organization_id"` // nil to unset
		} `in:"body=json"`
	}
)

// Response types for OpenAPI documentation
type SessionResponse struct {
	Session            *SessionInfo            `json:"session"`
	User               *UserInfo               `json:"user"`
	ActiveOrganization *ActiveOrganizationInfo `json:"active_organization,omitempty"`
}

type SessionInfo struct {
	ID                   string  `json:"id"`
	Token                string  `json:"token"`
	ActiveOrganizationID *string `json:"active_organization_id,omitempty"`
	ExpiresAt            string  `json:"expires_at"`
}

type UserInfo struct {
	ID            string  `json:"id"`
	Email         string  `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	Name          string  `json:"name"`
	Image         *string `json:"image,omitempty"`
}

type ActiveOrganizationInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	Logo       *string `json:"logo,omitempty"`
	MemberRole string  `json:"member_role"`
}

func SetupAuthHttpHandler(r chi.Router, authService authservice.Service) {
	h := authHttpHandler{
		authService: authService,
	}

	r.Route("/auth", func(r chi.Router) {
		// Public endpoints
		r.With(bodyparser.New[registerRequest]()).Post("/register", h.register)
		r.With(bodyparser.New[loginRequest]()).Post("/login", h.login)

		// Protected endpoints (require session)
		r.Group(func(r chi.Router) {
			r.Use(h.requireSession)
			r.Post("/logout", h.logout)
			r.Get("/session", h.getSession)
			r.Post("/session/refresh", h.refreshSession)
			r.Get("/sessions", h.listSessions)
			r.Delete("/sessions/{sessionId}", h.revokeSession)
			r.Delete("/sessions", h.revokeAllSessions)
			r.With(bodyparser.New[setActiveOrganizationRequest]()).Post("/session/active-organization", h.setActiveOrganization)
		})
	})
}

// register handles user registration
func (h *authHttpHandler) register(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[registerRequest](r).Body

	sessionData, err := h.authService.Register(r.Context(), authservice.RegisterInput{
		Email:    body.Email,
		Password: body.Password,
		Name:     body.Name,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toSessionResponse(sessionData), nil)
}

// login handles user login
func (h *authHttpHandler) login(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[loginRequest](r).Body

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	sessionData, err := h.authService.Login(r.Context(), authservice.LoginInput{
		Email:     body.Email,
		Password:  body.Password,
		IPAddress: &ipAddress,
		UserAgent: &userAgent,
	})
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	helper.Resp(w, toSessionResponse(sessionData), nil)
}

// logout handles user logout
func (h *authHttpHandler) logout(w http.ResponseWriter, r *http.Request) {
	token := getSessionToken(r)
	err := h.authService.Logout(r.Context(), token)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// getSession retrieves the current session
func (h *authHttpHandler) getSession(w http.ResponseWriter, r *http.Request) {
	sessionData := getSessionFromContext(r)
	helper.Resp(w, toSessionResponse(sessionData), nil)
}

// refreshSession extends the session expiration
func (h *authHttpHandler) refreshSession(w http.ResponseWriter, r *http.Request) {
	token := getSessionToken(r)
	sessionData, err := h.authService.RefreshSession(r.Context(), token)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, toSessionResponse(sessionData), nil)
}

// listSessions lists all active sessions for the current user
func (h *authHttpHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	sessionData := getSessionFromContext(r)
	sessions, err := h.authService.ListSessions(r.Context(), sessionData.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}

	result := make([]map[string]any, len(sessions))
	for i, s := range sessions {
		result[i] = map[string]any{
			"id":                     s.ID,
			"active_organization_id": s.ActiveOrganizationID,
			"ip_address":             s.IPAddress,
			"user_agent":             s.UserAgent,
			"expires_at":             s.ExpiresAt,
			"created_at":             s.CreatedAt,
		}
	}
	helper.Resp(w, result, nil)
}

// revokeSession revokes a specific session
func (h *authHttpHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	err := h.authService.RevokeSession(r.Context(), sessionID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// revokeAllSessions revokes all sessions for the current user
func (h *authHttpHandler) revokeAllSessions(w http.ResponseWriter, r *http.Request) {
	sessionData := getSessionFromContext(r)
	err := h.authService.RevokeAllSessions(r.Context(), sessionData.User.ID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, map[string]bool{"success": true}, nil)
}

// setActiveOrganization sets the active organization for the session
func (h *authHttpHandler) setActiveOrganization(w http.ResponseWriter, r *http.Request) {
	body := bodyparser.GetBodyFromContext[setActiveOrganizationRequest](r).Body
	token := getSessionToken(r)

	sessionData, err := h.authService.SetActiveOrganization(r.Context(), token, body.OrganizationID)
	if err != nil {
		helper.Resp(w, nil, err)
		return
	}
	helper.Resp(w, toSessionResponse(sessionData), nil)
}

// Middleware to require a valid session
func (h *authHttpHandler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getSessionToken(r)
		if token == "" {
			helper.Resp(w, nil, apperr.New(401, "missing_authorization"))
			return
		}

		sessionData, err := h.authService.GetSession(r.Context(), token)
		if err != nil {
			helper.Resp(w, nil, err)
			return
		}

		// Add session to context
		ctx := setSessionInContext(r.Context(), sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getSessionToken extracts the session token from the Authorization header
func getSessionToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	// Support "Bearer <token>" format
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return auth
}

// toSessionResponse converts internal session data to API response
func toSessionResponse(sd *authservice.SessionData) *SessionResponse {
	if sd == nil {
		return nil
	}

	resp := &SessionResponse{
		Session: &SessionInfo{
			ID:                   sd.Session.ID,
			Token:                sd.Session.Token,
			ActiveOrganizationID: sd.Session.ActiveOrganizationID,
			ExpiresAt:            sd.Session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		User: &UserInfo{
			ID:            sd.User.ID,
			Email:         sd.User.Email,
			EmailVerified: sd.User.EmailVerified,
			Name:          sd.User.Name,
			Image:         sd.User.Image,
		},
	}

	if sd.ActiveOrganization != nil {
		resp.ActiveOrganization = &ActiveOrganizationInfo{
			ID:         sd.ActiveOrganization.ID,
			Name:       sd.ActiveOrganization.Name,
			Slug:       sd.ActiveOrganization.Slug,
			Logo:       sd.ActiveOrganization.Logo,
			MemberRole: sd.ActiveOrganization.MemberRole,
		}
	}

	return resp
}
