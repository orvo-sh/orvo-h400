package helper

import (
	"encoding/json"
	"net/http"

	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

func Resp(w http.ResponseWriter, val any, err apperr.Error) {
	w.Header().Set("Content-Type", "application/json")

	var resp map[string]any
	statusCode := http.StatusOK

	if err != nil {
		resp = map[string]any{
			"status": "error",
			"code":   err.Code(),
		}
		statusCode = err.Status()
		w.WriteHeader(statusCode)
	} else {
		resp = map[string]any{
			"status": "success",
			"data":   val,
		}
	}

	util.Must(0, json.NewEncoder(w).Encode(resp))
}
