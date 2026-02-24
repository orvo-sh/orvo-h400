package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/dashboardservice"
	"github.com/orvo-sh/orvo/internal/domain/services/logservice"
	"github.com/orvo-sh/orvo/internal/domain/services/metricservice"
	"github.com/orvo-sh/orvo/internal/domain/services/organizationservice"
	"github.com/orvo-sh/orvo/internal/domain/services/traceservice"
	"github.com/orvo-sh/orvo/internal/domain/workers"
	"github.com/orvo-sh/orvo/internal/http/handlers"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/ingest"
	"github.com/orvo-sh/orvo/internal/logger"
	appotel "github.com/orvo-sh/orvo/internal/otel"
	"github.com/orvo-sh/orvo/pkg/background"
	"github.com/orvo-sh/orvo/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	godotenv.Load(".env")

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := util.Must(config.Load())

	otelShutdown, err := appotel.Init(rootCtx, appotel.Config{
		ServiceName:  "orvo-app",
		Environment:  cfg.App.Environment,
		OTLPEndpoint: cfg.Otel.Endpoint,
		APIKey:       cfg.Otel.ApiKey,
	})
	if err != nil {
		panic(err)
	}
	defer otelShutdown()

	appLogger := logger.New(logger.Config{
		ServiceName: "orvo-app",
		Environment: cfg.App.Environment,
	})

	pg := util.Must(postgres.New(rootCtx, postgres.Config{
		URL: cfg.Postgres.URL,
	}))
	defer pg.Close()

	backgroundManager := background.New(appLogger, background.Config{
		DefaultTimeout: 30 * time.Second,
	})

	authService := authservice.New(pg, appLogger, backgroundManager, authservice.Config{
		SessionExpiresIn:       7 * 24 * time.Hour,
		SessionUpdateAge:       24 * time.Hour,
		ApiKeyCacheResolverTTL: 60 * time.Second,
	})
	organizationService := organizationservice.New(pg, appLogger, organizationservice.Config{
		MaxOrganizationsPerUser: 10,
	})
	logSvc := logservice.New(pg, appLogger, logservice.Config{
		HotRetention: cfg.Telemetry.HotRetentionLogs,
	})
	traceSvc := traceservice.New(pg, appLogger, traceservice.Config{
		HotRetention: cfg.Telemetry.HotRetentionTraces,
	})
	metricSvc := metricservice.New(pg, appLogger)
	dashboardSvc := dashboardservice.New(pg, appLogger)

	partitionManager := workers.NewPartitionManager(appLogger, pg.Pool())
	startupCtx, startupCancel := context.WithTimeout(rootCtx, 45*time.Second)
	if err := partitionManager.EnsureFuturePartitions(startupCtx, cfg.Telemetry.PartitionPrecreateDays); err != nil {
		startupCancel()
		panic(err)
	}
	startupCancel()

	objectStore, err := workers.NewObjectStore(rootCtx, workers.ObjectStoreConfig{
		Bucket:       cfg.S3.Bucket,
		Region:       cfg.S3.Region,
		Endpoint:     cfg.S3.Endpoint,
		AccessKeyID:  cfg.S3.AccessKeyID,
		SecretKey:    cfg.S3.SecretKey,
		UsePathStyle: cfg.S3.UsePathStyle,
	})
	if err != nil {
		panic(err)
	}

	archiveManager := workers.NewArchiveManager(appLogger, pg.Pool(), objectStore, workers.ArchiveManagerConfig{
		Prefix:                  cfg.S3.Prefix,
		HotRetentionLogs:        cfg.Telemetry.HotRetentionLogs,
		HotRetentionTraces:      cfg.Telemetry.HotRetentionTraces,
		HotRetentionMetrics:     cfg.Telemetry.HotRetentionMetrics,
		ArchiveRetentionLogs:    cfg.Telemetry.ArchiveRetentionLogs,
		ArchiveRetentionTraces:  cfg.Telemetry.ArchiveRetentionTraces,
		ArchiveRetentionMetrics: cfg.Telemetry.ArchiveRetentionMetrics,
	})

	restoreManager := workers.NewRestoreManager(appLogger, pg.Pool(), objectStore, partitionManager, workers.RestoreManagerConfig{
		RestoredTTL:          cfg.Telemetry.RestoredTTL,
		RestoreThroughputBPS: cfg.Telemetry.RestoreThroughputBPS,
	})

	workerManager := workers.NewManager(appLogger, pg.Pool(), workers.ManagerConfig{
		DefaultTimeout: 60 * time.Second,
		Timezone:       time.UTC,
	})

	if cfg.App.EnableWorkers {
		registerWorkers(workerManager, partitionManager, archiveManager, restoreManager, cfg)
		workerManager.Start()
		restoreManager.Notify()
		defer workerManager.Stop()
	}

	ingestServer, err := ingest.NewServer(pg, authService, appLogger, ingest.ServerConfig{
		EnableGRPC: cfg.App.EnableOTLPGRPC,
		EnableHTTP: cfg.App.EnableOTLPHTTP,
		GRPCPort:   cfg.App.OtlpGRPCPort,
		HTTPPort:   cfg.App.OtlpHTTPPort,
	})
	if err != nil {
		panic(err)
	}
	ingestServer.Start()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ingestServer.Shutdown(shutdownCtx)
	}()

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "http.request")
	})

	handlers.FrontendHandler(router)
	router.With(
		cors.Handler(cors.Options{
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			AllowOriginFunc: func(_ *http.Request, _ string) bool {
				return true
			},
			AllowCredentials: true,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		})).
		Route("/api/v1", func(r chi.Router) {
			r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte("OK"))
			})

			humaConfig := huma.DefaultConfig("orvo", "1.0.0")
			humaConfig.Servers = []*huma.Server{
				{URL: "/api/v1"},
			}

			api := humachi.New(r, humaConfig)

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
			handlers.NewLogHandler(logSvc, authService).RegisterRoutes(api)
			handlers.NewTraceHandler(traceSvc, authService).RegisterRoutes(api)
			handlers.NewMetricHandler(metricSvc, authService).RegisterRoutes(api)
			handlers.NewDashboardHandler(dashboardSvc, authService).RegisterRoutes(api)
			handlers.NewArchiveHandler(authService, restoreManager).RegisterRoutes(api)

			r.Get("/*", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			})
		})

	if !cfg.App.EnableAPI {
		appLogger.Info("API listener disabled; background workers and ingest listeners are running")
		<-rootCtx.Done()
		return
	}

	httpServer := &http.Server{
		Addr:    ":" + cfg.App.AppPort,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		appLogger.Info("starting API server", slog.String("port", cfg.App.AppPort))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-rootCtx.Done():
	case err := <-errCh:
		appLogger.Error("API server stopped with error", slog.Any("error", err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("failed to shutdown API server", slog.Any("error", err))
		os.Exit(1)
	}
}

func registerWorkers(
	manager *workers.Manager,
	partitionManager *workers.PartitionManager,
	archiveManager *workers.ArchiveManager,
	restoreManager *workers.RestoreManager,
	cfg *config.Config,
) {
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	must(manager.RegisterCron("partition-precreate", cfg.Workers.PartitionPrecreate, func(ctx context.Context) error {
		return partitionManager.EnsureFuturePartitions(ctx, cfg.Telemetry.PartitionPrecreateDays)
	}))

	must(manager.RegisterCron("hot-retention-prune", cfg.Workers.HotRetention, func(ctx context.Context) error {
		return partitionManager.ApplyHotRetention(
			ctx,
			cfg.Telemetry.HotRetentionLogs,
			cfg.Telemetry.HotRetentionTraces,
			cfg.Telemetry.HotRetentionMetrics,
		)
	}))

	must(manager.RegisterCron("restore-ttl-cleaner", cfg.Workers.RestoreTTL, func(ctx context.Context) error {
		return partitionManager.ApplyRestoredTTL(ctx, cfg.Telemetry.RestoredTTL)
	}))

	must(manager.RegisterCron("archive-export", cfg.Workers.ArchiveExport, func(ctx context.Context) error {
		return archiveManager.ExportDue(ctx)
	}))

	must(manager.RegisterCron("archive-retention", cfg.Workers.ArchiveRetention, func(ctx context.Context) error {
		return archiveManager.ApplyRetention(ctx)
	}))

	must(manager.RegisterCron("restore-queue-poller", cfg.Workers.RestoreQueuePoll, func(ctx context.Context) error {
		return restoreManager.ProcessQueued(ctx)
	}))

	manager.RegisterChannel("restore-processor", restoreManager.TriggerChan(), func(ctx context.Context) error {
		return restoreManager.ProcessQueued(ctx)
	})
}
