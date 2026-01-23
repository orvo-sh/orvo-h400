package bodyparser

import (
	"context"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/orvo-sh/orvo/internal/http/helper"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

type bodyContextKey string

const bodyKey bodyContextKey = "req_body"

func New[T any]() func(next http.Handler) http.Handler {
	var prototype T
	engine := util.Must(httpin.New(&prototype))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			input, err := engine.Decode(r)
			if err != nil {
				helper.Resp(w, nil, apperr.ErrBadRequest)
				return
			}
			ctx := context.WithValue(r.Context(), bodyKey, input)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetBodyFromContext[T any](r *http.Request) T {
	if bodyObj, ok := r.Context().Value(bodyKey).(*T); ok {
		return *bodyObj
	}

	panic("bodyparser: body not found in context or incorrect type")
}
