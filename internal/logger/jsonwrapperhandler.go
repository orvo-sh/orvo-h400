package logger

import (
	"context"
	"encoding/json"
	"log/slog"
)

type jsonWrapperHandler struct {
	slog.Handler
}

func (h *jsonWrapperHandler) Handle(ctx context.Context, r slog.Record) error {
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)

	r.Attrs(func(a slog.Attr) bool {
		switch a.Value.Kind() {
		case slog.KindAny:
			if j, err := json.Marshal(a.Value.Any()); err == nil {
				newRecord.AddAttrs(slog.String(a.Key, string(j)))
			} else {
				newRecord.AddAttrs(a)
			}
		default:
			newRecord.AddAttrs(a)
		}
		return true
	})

	return h.Handler.Handle(ctx, newRecord)
}
