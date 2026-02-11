package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/ingestservice"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/sink"
	"github.com/orvo-sh/orvo/pkg/background"
	"github.com/orvo-sh/orvo/pkg/util"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := util.Must(config.Load())

	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{})).With(
		slog.String("service", "ingest"),
		slog.String("environment", cfg.App.Environment),
	)

	// Infrastructure.
	pg := util.Must(postgres.New(ctx, postgres.Config{
		URL: cfg.Postgres.URL,
	}))
	defer pg.Close()

	ch := util.Must(clickhouse.New(ctx, clickhouse.Config{
		Address:  cfg.Clickhouse.Address,
		Database: cfg.Clickhouse.Database,
		User:     cfg.Clickhouse.User,
		Password: cfg.Clickhouse.Password,
	}))
	defer ch.Close()

	backgroundManager := background.New(logger, background.Config{
		DefaultTimeout: 30 * time.Second,
	})

	authService := authservice.New(pg, logger, backgroundManager, authservice.Config{
		SessionExpiresIn:       7 * 24 * time.Hour,
		SessionUpdateAge:       24 * time.Hour,
		ApiKeyCacheResolverTTL: 60 * time.Second,
	})

	logSink := sink.NewLogSink(ch, logger)

	ingestService := ingestservice.New(logSink, logger)

	// OTLP/gRPC receiver (:4317).
	grpcServer := grpc.NewServer()
	collectorpb.RegisterLogsServiceServer(grpcServer, &grpcHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	})

	grpcListener := util.Must(net.Listen("tcp", ":4317"))
	go func() {
		logger.Info("OTLP/gRPC receiver listening on :4317")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Error("gRPC server error", slog.Any("error", err))
		}
	}()

	// OTLP/HTTP receiver (:4318).
	httpMux := http.NewServeMux()
	httpHandler := &httpLogHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	}
	httpMux.Handle("/v1/logs", httpHandler)
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    ":4318",
		Handler: httpMux,
	}
	go func() {
		logger.Info("OTLP/HTTP receiver listening on :4318")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", slog.Any("error", err))
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down ingest service...")

	grpcServer.GracefulStop()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)

	if err := logSink.Close(); err != nil {
		logger.Error("failed to close log sink", slog.Any("error", err))
	}

	logger.Info("ingest service stopped")
}

// extractApiKey pulls the API key from Authorization header or x-api-key.
func extractApiKey(authorization string, apiKeyHeader string) string {
	if apiKeyHeader != "" {
		return apiKeyHeader
	}
	if strings.HasPrefix(authorization, "Bearer ") {
		return strings.TrimPrefix(authorization, "Bearer ")
	}
	return ""
}

// --- gRPC handler ---

type grpcHandler struct {
	collectorpb.UnimplementedLogsServiceServer
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *grpcHandler) Export(ctx context.Context, req *collectorpb.ExportLogsServiceRequest) (*collectorpb.ExportLogsServiceResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	var rawKey string
	if vals := md.Get("authorization"); len(vals) > 0 {
		rawKey = extractApiKey(vals[0], "")
	}
	if rawKey == "" {
		if vals := md.Get("x-api-key"); len(vals) > 0 {
			rawKey = vals[0]
		}
	}
	if rawKey == "" {
		return nil, status.Error(codes.Unauthenticated, "missing API key")
	}

	orgID, appErr := h.authService.ResolveApiKey(ctx, rawKey)
	if appErr != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	if err := h.ingestService.IngestLogs(ctx, ingestservice.IngestLogsInput{
		OrganizationID: *orgID,
		ResourceLogs:   req.GetResourceLogs(),
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to ingest logs")
	}

	return &collectorpb.ExportLogsServiceResponse{}, nil
}

// --- HTTP handler ---

type httpLogHandler struct {
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *httpLogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawKey := extractApiKey(r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"))
	if rawKey == "" {
		http.Error(w, "missing API key", http.StatusUnauthorized)
		return
	}

	orgID, appErr := h.authService.ResolveApiKey(r.Context(), rawKey)
	if appErr != nil {
		http.Error(w, "invalid API key", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &collectorpb.ExportLogsServiceRequest{}

	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "application/x-protobuf"):
		if err := proto.Unmarshal(body, req); err != nil {
			http.Error(w, "failed to unmarshal protobuf", http.StatusBadRequest)
			return
		}
	case strings.Contains(contentType, "application/json"):
		if err := json.Unmarshal(body, req); err != nil {
			http.Error(w, "failed to unmarshal json", http.StatusBadRequest)
			return
		}
	default:
		// Default to protobuf per OTLP spec.
		if err := proto.Unmarshal(body, req); err != nil {
			http.Error(w, "failed to unmarshal request", http.StatusBadRequest)
			return
		}
	}

	if ingestErr := h.ingestService.IngestLogs(r.Context(), ingestservice.IngestLogsInput{
		OrganizationID: *orgID,
		ResourceLogs:   req.GetResourceLogs(),
	}); ingestErr != nil {
		http.Error(w, "failed to ingest logs", http.StatusInternalServerError)
		return
	}

	resp := &collectorpb.ExportLogsServiceResponse{}

	switch {
	case strings.Contains(contentType, "application/json"):
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	default:
		w.Header().Set("Content-Type", "application/x-protobuf")
		out, _ := proto.Marshal(resp)
		w.Write(out)
	}
}
