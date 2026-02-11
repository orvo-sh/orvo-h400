package logger

import (
	"context"
	"log/slog"
)

type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(h.addRequestID(ctx)...)
	return h.Handler.Handle(ctx, r)
}
func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}
func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}

func (h contextHandler) addRequestID(ctx context.Context) (as []slog.Attr) {
	span, ok := ctx.Value("requestid.ContextKey").(string)
	if ok {
		as = append(as, slog.Attr{Key: "request_id", Value: slog.StringValue(span)})
	}
	return
}
