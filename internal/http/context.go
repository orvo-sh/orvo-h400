package http

import (
	"context"
	"net/http"

	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
)

type contextKey string

const (
	sessionContextKey contextKey = "session"
)

// setSessionInContext stores the session data in the request context
func setSessionInContext(ctx context.Context, session *authservice.SessionData) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// getSessionFromContext retrieves the session data from the request context
func getSessionFromContext(r *http.Request) *authservice.SessionData {
	session, ok := r.Context().Value(sessionContextKey).(*authservice.SessionData)
	if !ok {
		return nil
	}
	return session
}

// MustGetSessionFromContext retrieves the session data from the context, panics if not found
func MustGetSessionFromContext(r *http.Request) *authservice.SessionData {
	session := getSessionFromContext(r)
	if session == nil {
		panic("session not found in context - ensure requireSession middleware is used")
	}
	return session
}
