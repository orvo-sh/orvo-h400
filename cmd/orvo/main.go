package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/logservice"
	"github.com/orvo-sh/orvo/internal/domain/services/organizationservice"
	"github.com/orvo-sh/orvo/internal/domain/services/traceservice"
	"github.com/orvo-sh/orvo/internal/http/handlers"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/infra/redis"
	"github.com/orvo-sh/orvo/internal/logger"
	appotel "github.com/orvo-sh/orvo/internal/otel"
	"github.com/orvo-sh/orvo/pkg/background"
	"github.com/orvo-sh/orvo/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := util.Must(config.Load())

	if cfg.Otel.ApiKey == "" {
		cfg.Otel.ApiKey = "CGIGs8wiB9DRO6fARVIRlhES4ZtNhSVa0_GxsA-fj61h8fxuKUtGZFaTIChfzsId"
	}

	// Initialize OpenTelemetry tracing (TracerProvider + OTLP exporter).
	otelShutdown, otelRes, err := appotel.Init(ctx, appotel.Config{
		ServiceName:  "orvo-app",
		Environment:  cfg.App.Environment,
		OTLPEndpoint: cfg.Otel.Endpoint,
		APIKey:       cfg.Otel.ApiKey,
	})
	if err != nil {
		panic(err)
	}
	defer otelShutdown()

	logger, cleanup, err := logger.New(ctx, logger.Config{
		ServiceName:  "orvo-app",
		Environment:  cfg.App.Environment,
		OTLPEndpoint: cfg.Otel.Endpoint,
		APIKey:       cfg.Otel.ApiKey,
		Resource:     otelRes,
	})
	if err != nil {
		panic(err)
	}
	defer cleanup()

	postgres := util.Must(postgres.New(ctx, postgres.Config{
		URL: cfg.Postgres.URL,
	}))
	defer postgres.Close()

	clickhouse := util.Must(clickhouse.New(ctx, clickhouse.Config{
		Address:  cfg.Clickhouse.Address,
		Database: cfg.Clickhouse.Database,
		User:     cfg.Clickhouse.User,
		Password: cfg.Clickhouse.Password,
	}))
	defer clickhouse.Close()

	redis := util.Must(redis.New(ctx, redis.Config{
		Address:  cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}))
	defer redis.Close()

	r := chi.NewRouter()

	// OpenTelemetry HTTP middleware — creates a span for every request.
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "http.request")
	})

	backgroundManager := background.New(logger, background.Config{
		DefaultTimeout: 30 * time.Second,
	})

	authService := authservice.New(postgres, logger, backgroundManager, authservice.Config{
		SessionExpiresIn:       7 * 24 * time.Hour,
		SessionUpdateAge:       24 * time.Hour,
		ApiKeyCacheResolverTTL: 60 * time.Second,
	})
	organizationService := organizationservice.New(postgres, logger, organizationservice.Config{
		MaxOrganizationsPerUser: 10,
	})
	logService := logservice.New(clickhouse, logger)
	traceService := traceservice.New(clickhouse, logger)

	handlers.FrontendHandler(r)
	r.With(
		cors.Handler(cors.Options{
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			AllowOriginFunc: func(r *http.Request, origin string) bool {
				return true
			},
			AllowCredentials: true,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		})).
		Route("/api/v1", func(r chi.Router) {
			r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			})

			config := huma.DefaultConfig("orvo", "1.0.0")
			config.Servers = []*huma.Server{
				{URL: "/api/v1"},
			}

			api := humachi.New(r, config)

			authmiddleware.Init(authmiddleware.Config{
				SessionCookieKey: "orvo_sess",
			})

			handlers.NewAuthHandler(authService, handlers.NewAuthConfig{
				SessionCookieKey:       "orvo_sess",
				SessionCookieDomain:    "",
				SessionCookieSecure:    false,
				SessionCookieSameSite:  http.SameSiteLaxMode,
				SessionCookieExpiresIn: 7 * 24 * time.Hour,
			}).RegisterRoutes(api)
			handlers.NewOrganizationHandler(organizationService, authService).RegisterRoutes(api)
			handlers.NewApiKeyHandler(authService).RegisterRoutes(api)
			handlers.NewLogHandler(logService, authService).RegisterRoutes(api)
			handlers.NewTraceHandler(traceService, authService).RegisterRoutes(api)

			r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			})
		})

	logger.Info("starting server on port " + cfg.App.AppPort)
	if err := http.ListenAndServe(":"+cfg.App.AppPort, r); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
}
