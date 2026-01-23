package main

import (
	"log/slog"
	"net/http"
	"os"

	httpin_integration "github.com/ggicci/httpin/integration"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/lmittmann/tint"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/services/ingestservice"
	http_handler "github.com/orvo-sh/orvo/internal/http"
	"github.com/orvo-sh/orvo/internal/infra/redis"
	"github.com/orvo-sh/orvo/pkg/util"
)

func main() {
	config := util.Must(config.Load())

	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{})).With(
		slog.String("service", "ingest"),
		slog.String("environment", config.App.Environment),
	)

	redisClient := util.Must(redis.New(redis.Config{
		Address:  config.Redis.Address,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	}))
	defer redisClient.Close()

	r := chi.NewRouter()
	httpin_integration.UseGochiURLParam("path", chi.URLParam)

	ingestService := ingestservice.New(logger, redisClient)

	r.With(cors.Handler(cors.Options{
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return true
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
	})).Route("/", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		})

		http_handler.SetupIngestHttpHandler(r, ingestService)

		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
	})

	logger.Info("starting ingest server on port " + config.App.IngestPort)
	if err := http.ListenAndServe(":"+config.App.IngestPort, r); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
}
