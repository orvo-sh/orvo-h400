package helpers

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func ErrResp(ctx huma.Context, api huma.API, err apperr.Error) {
	huma.WriteErr(api, ctx, err.Status(), err.Code())
}
