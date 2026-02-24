package authmiddleware

import (
	"context"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/http/helpers"
)

type contextKey string

const SessionContextKey contextKey = "orvo_sess"

type Config struct {
	SessionCookieKey string
	inited           bool
}

var cfg Config

func Init(config Config) {
	cfg = config
	cfg.inited = true
}

func New(api huma.API, authService authservice.Service) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {

		if !cfg.inited {
			panic("authmiddleware: authmiddleware.Init must be called before usage of middleware")
		}

		var token string

		// Try Authorization header first
		auth := ctx.Header("Authorization")
		if auth != "" {
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			} else {
				token = auth
			}
		}

		// Fall back to session cookie for browser-based auth
		if token == "" {
			cookie := helpers.GetCookie(ctx, cfg.SessionCookieKey)
			if cookie != "" {
				token = cookie
			}
		}

		if token == "" {
			helpers.ErrResp(ctx, api, errs.ErrMissingAuthorization)
			return
		}

		session, err := authService.GetSession(ctx.Context(), token)
		if err != nil {
			helpers.ErrResp(ctx, api, err)
			return
		}

		if orgID := strings.TrimSpace(ctx.Param("organization_id")); orgID != "" {
			if err := authService.EnsureOrganizationMember(ctx.Context(), session.UserID, orgID); err != nil {
				helpers.ErrResp(ctx, api, err)
				return
			}
		}

		ctx = huma.WithValue(ctx, SessionContextKey, session)
		next(ctx)
	}
}

func GetSessionFromContext(ctx context.Context) *models.Session {
	if v, ok := ctx.Value(SessionContextKey).(*models.Session); ok {
		return v
	}
	panic("session not found in context - ensure RequireSessionMiddleware is applied")
}
