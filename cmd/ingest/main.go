package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/ingestservice"
	"github.com/orvo-sh/orvo/internal/infra/clickhouse"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/sink"
	"github.com/orvo-sh/orvo/pkg/background"
	"github.com/orvo-sh/orvo/pkg/util"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	godotenv.Load(".env")

	cfg := util.Must(config.Load())

	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{})).With(
		slog.String("service", "ingest"),
		slog.String("environment", cfg.App.Environment),
	)

	pg := util.Must(postgres.New(ctx, postgres.Config{
		URL: cfg.Postgres.URL,
	}))
	defer pg.Close()

	ch := util.Must(clickhouse.New(ctx, clickhouse.Config{
		URL: cfg.Clickhouse.URL,
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
	spanSink := sink.NewSpanSink(ch, logger)
	metricSink := sink.NewMetricSink(ch, logger)

	ingestService := ingestservice.New(logSink, spanSink, metricSink, logger)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	collectorlogspb.RegisterLogsServiceServer(grpcServer, &grpcLogHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	})
	collectortracepb.RegisterTraceServiceServer(grpcServer, &grpcTraceHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	})
	collectormetricspb.RegisterMetricsServiceServer(grpcServer, &grpcMetricHandler{
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

	httpMux := http.NewServeMux()
	httpLogH := &httpLogHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	}
	httpTraceH := &httpTraceHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	}
	httpMetricH := &httpMetricHandler{
		authService:   authService,
		ingestService: ingestService,
		logger:        logger,
	}
	httpMux.Handle("/v1/logs", httpLogH)
	httpMux.Handle("/v1/traces", httpTraceH)
	httpMux.Handle("/v1/metrics", httpMetricH)
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    ":4318",
		Handler: otelhttp.NewHandler(httpMux, "ingest.http"),
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
	if err := spanSink.Close(); err != nil {
		logger.Error("failed to close span sink", slog.Any("error", err))
	}
	if err := metricSink.Close(); err != nil {
		logger.Error("failed to close metric sink", slog.Any("error", err))
	}

	logger.Info("ingest service stopped")
}

func shouldLoadDotEnv() bool {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENVIRONMENT")))
	return environment == "" || environment == "development" || environment == "dev" || environment == "local"
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

// resolveOrgFromGRPC extracts the API key from gRPC metadata and resolves the organization ID.
func resolveOrgFromGRPC(ctx context.Context, authService authservice.Service) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
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
		return "", status.Error(codes.Unauthenticated, "missing API key")
	}

	orgID, appErr := authService.ResolveApiKey(ctx, rawKey)
	if appErr != nil {
		return "", status.Error(codes.Unauthenticated, "invalid API key")
	}
	return *orgID, nil
}

// resolveOrgFromHTTP extracts the API key from HTTP headers and resolves the organization ID.
func resolveOrgFromHTTP(r *http.Request, authService authservice.Service) (string, error) {
	rawKey := extractApiKey(r.Header.Get("Authorization"), r.Header.Get("X-Api-Key"))
	if rawKey == "" {
		return "", fmt.Errorf("missing API key")
	}

	orgID, appErr := authService.ResolveApiKey(r.Context(), rawKey)
	if appErr != nil {
		return "", fmt.Errorf("invalid API key")
	}
	return *orgID, nil
}

// --- gRPC Log handler ---

type grpcLogHandler struct {
	collectorlogspb.UnimplementedLogsServiceServer
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *grpcLogHandler) Export(ctx context.Context, req *collectorlogspb.ExportLogsServiceRequest) (*collectorlogspb.ExportLogsServiceResponse, error) {
	orgID, err := resolveOrgFromGRPC(ctx, h.authService)
	if err != nil {
		return nil, err
	}

	if ingestErr := h.ingestService.IngestLogs(ctx, ingestservice.IngestLogsInput{
		OrganizationID: orgID,
		ResourceLogs:   req.GetResourceLogs(),
	}); ingestErr != nil {
		return nil, status.Error(codes.Internal, "failed to ingest logs")
	}

	return &collectorlogspb.ExportLogsServiceResponse{}, nil
}

// --- gRPC Trace handler ---

type grpcTraceHandler struct {
	collectortracepb.UnimplementedTraceServiceServer
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *grpcTraceHandler) Export(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	orgID, err := resolveOrgFromGRPC(ctx, h.authService)
	if err != nil {
		return nil, err
	}

	if ingestErr := h.ingestService.IngestTraces(ctx, ingestservice.IngestTracesInput{
		OrganizationID: orgID,
		ResourceSpans:  req.GetResourceSpans(),
	}); ingestErr != nil {
		return nil, status.Error(codes.Internal, "failed to ingest traces")
	}

	return &collectortracepb.ExportTraceServiceResponse{}, nil
}

// --- HTTP Log handler ---

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

	orgID, err := resolveOrgFromHTTP(r, h.authService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &collectorlogspb.ExportLogsServiceRequest{}

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
		OrganizationID: orgID,
		ResourceLogs:   req.GetResourceLogs(),
	}); ingestErr != nil {
		http.Error(w, "failed to ingest logs", http.StatusInternalServerError)
		return
	}

	resp := &collectorlogspb.ExportLogsServiceResponse{}

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

// --- HTTP Trace handler ---

type httpTraceHandler struct {
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *httpTraceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID, err := resolveOrgFromHTTP(r, h.authService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &collectortracepb.ExportTraceServiceRequest{}

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
		if err := proto.Unmarshal(body, req); err != nil {
			http.Error(w, "failed to unmarshal request", http.StatusBadRequest)
			return
		}
	}

	if ingestErr := h.ingestService.IngestTraces(r.Context(), ingestservice.IngestTracesInput{
		OrganizationID: orgID,
		ResourceSpans:  req.GetResourceSpans(),
	}); ingestErr != nil {
		http.Error(w, "failed to ingest traces", http.StatusInternalServerError)
		return
	}

	resp := &collectortracepb.ExportTraceServiceResponse{}

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

// --- gRPC Metric handler ---

type grpcMetricHandler struct {
	collectormetricspb.UnimplementedMetricsServiceServer
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *grpcMetricHandler) Export(ctx context.Context, req *collectormetricspb.ExportMetricsServiceRequest) (*collectormetricspb.ExportMetricsServiceResponse, error) {
	orgID, err := resolveOrgFromGRPC(ctx, h.authService)
	if err != nil {
		return nil, err
	}

	if ingestErr := h.ingestService.IngestMetrics(ctx, ingestservice.IngestMetricsInput{
		OrganizationID:  orgID,
		ResourceMetrics: req.GetResourceMetrics(),
	}); ingestErr != nil {
		return nil, status.Error(codes.Internal, "failed to ingest metrics")
	}

	return &collectormetricspb.ExportMetricsServiceResponse{}, nil
}

// --- HTTP Metric handler ---

type httpMetricHandler struct {
	authService   authservice.Service
	ingestService ingestservice.Service
	logger        *slog.Logger
}

func (h *httpMetricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orgID, err := resolveOrgFromHTTP(r, h.authService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req := &collectormetricspb.ExportMetricsServiceRequest{}

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
		if err := proto.Unmarshal(body, req); err != nil {
			http.Error(w, "failed to unmarshal request", http.StatusBadRequest)
			return
		}
	}

	if ingestErr := h.ingestService.IngestMetrics(r.Context(), ingestservice.IngestMetricsInput{
		OrganizationID:  orgID,
		ResourceMetrics: req.GetResourceMetrics(),
	}); ingestErr != nil {
		http.Error(w, "failed to ingest metrics", http.StatusInternalServerError)
		return
	}

	resp := &collectormetricspb.ExportMetricsServiceResponse{}

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
