package handlers

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	frontendfs "github.com/orvo-sh/orvo/frontend"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
)

type healthCheckStatus struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type readinessResponse struct {
	Status    string                       `json:"status"`
	Timestamp string                       `json:"timestamp"`
	Checks    map[string]healthCheckStatus `json:"checks"`
}

func RegisterHealthRoutes(r chi.Router, pg *postgres.DB, dockerBinary string) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
	})

	r.Get("/health/ready", func(w http.ResponseWriter, req *http.Request) {
		response := readinessResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Checks:    map[string]healthCheckStatus{},
		}

		checkCtx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()

		if err := pg.Pool().Ping(checkCtx); err != nil {
			response.Status = "error"
			response.Checks["database"] = healthCheckStatus{
				Status: "error",
				Detail: err.Error(),
			}
		} else {
			response.Checks["database"] = healthCheckStatus{Status: "ok"}
		}

		if _, err := fs.Stat(frontendfs.Efs, "build/200.html"); err != nil {
			response.Status = "error"
			response.Checks["frontend"] = healthCheckStatus{
				Status: "error",
				Detail: err.Error(),
			}
		} else {
			response.Checks["frontend"] = healthCheckStatus{Status: "ok"}
		}

		binary := strings.TrimSpace(dockerBinary)
		if binary == "" {
			binary = "docker"
		}
		cmd := exec.CommandContext(checkCtx, binary, "version", "--format", "{{.Server.Version}}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			response.Status = "error"
			response.Checks["docker"] = healthCheckStatus{
				Status: "error",
				Detail: strings.TrimSpace(string(output)),
			}
		} else {
			response.Checks["docker"] = healthCheckStatus{
				Status: "ok",
				Detail: strings.TrimSpace(string(output)),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if response.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(response)
	})
}
