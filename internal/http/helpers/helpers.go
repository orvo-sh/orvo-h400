package helpers

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func ErrResp(ctx huma.Context, api huma.API, err apperr.Error) {
	huma.WriteErr(api, ctx, err.Status(), err.Code())
}

// GetCookie reads a cookie value from the huma context.
// The huma.Context embeds an http.Request accessible via the Cookie header.
func GetCookie(ctx huma.Context, name string) string {
	header := ctx.Header("Cookie")
	if header == "" {
		return ""
	}
	// Parse the Cookie header manually
	req := &http.Request{Header: http.Header{"Cookie": {header}}}
	cookie, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
