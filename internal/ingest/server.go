package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/ingestservice"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/sink"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type ServerConfig struct {
	EnableGRPC bool
	EnableHTTP bool
	GRPCPort   string
	HTTPPort   string
}

type Server struct {
	logger        *slog.Logger
	logSink       *sink.LogSink
	spanSink      *sink.SpanSink
	metricSink    *sink.MetricSink
	grpcServer    *grpc.Server
	grpcListener  net.Listener
	httpServer    *http.Server
	ingestService ingestservice.Service
}

func NewServer(pg *postgres.DB, authService authservice.Service, logger *slog.Logger, cfg ServerConfig) (*Server, error) {
	if cfg.GRPCPort == "" {
		cfg.GRPCPort = "4317"
	}
	if cfg.HTTPPort == "" {
		cfg.HTTPPort = "4318"
	}

	server := &Server{
		logger: logger.With("component", "ingest"),
	}

	server.logSink = sink.NewLogSink(pg, logger)
	server.spanSink = sink.NewSpanSink(pg, logger)
	server.metricSink = sink.NewMetricSink(pg, logger)
	server.ingestService = ingestservice.New(server.logSink, server.spanSink, server.metricSink, logger)

	if cfg.EnableGRPC {
		server.grpcServer = grpc.NewServer(
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
		collectorlogspb.RegisterLogsServiceServer(server.grpcServer, &grpcLogHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})
		collectortracepb.RegisterTraceServiceServer(server.grpcServer, &grpcTraceHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})
		collectormetricspb.RegisterMetricsServiceServer(server.grpcServer, &grpcMetricHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})

		listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			return nil, fmt.Errorf("listen otlp grpc on %s: %w", cfg.GRPCPort, err)
		}
		server.grpcListener = listener
	}

	if cfg.EnableHTTP {
		httpMux := http.NewServeMux()
		httpMux.Handle("/v1/logs", &httpLogHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})
		httpMux.Handle("/v1/traces", &httpTraceHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})
		httpMux.Handle("/v1/metrics", &httpMetricHandler{
			authService:   authService,
			ingestService: server.ingestService,
			logger:        logger,
		})
		httpMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("OK"))
		})

		server.httpServer = &http.Server{
			Addr:    ":" + cfg.HTTPPort,
			Handler: otelhttp.NewHandler(httpMux, "ingest.http"),
		}
	}

	return server, nil
}

func (s *Server) Start() {
	if s.grpcServer != nil && s.grpcListener != nil {
		go func() {
			s.logger.Info("OTLP/gRPC receiver listening", slog.String("addr", s.grpcListener.Addr().String()))
			if err := s.grpcServer.Serve(s.grpcListener); err != nil {
				s.logger.Error("gRPC ingest server stopped", slog.Any("error", err))
			}
		}()
	}

	if s.httpServer != nil {
		go func() {
			s.logger.Info("OTLP/HTTP receiver listening", slog.String("addr", s.httpServer.Addr))
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("HTTP ingest server stopped", slog.Any("error", err))
			}
		}()
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Warn("failed to shutdown ingest HTTP server", slog.Any("error", err))
		}
	}
	if err := s.logSink.Close(); err != nil {
		s.logger.Warn("failed to close log sink", slog.Any("error", err))
	}
	if err := s.spanSink.Close(); err != nil {
		s.logger.Warn("failed to close span sink", slog.Any("error", err))
	}
	if err := s.metricSink.Close(); err != nil {
		s.logger.Warn("failed to close metric sink", slog.Any("error", err))
	}
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

func unmarshalOTLPJSON(body []byte, msg proto.Message) error {
	options := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	return options.Unmarshal(body, msg)
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
		if err := unmarshalOTLPJSON(body, req); err != nil {
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
		if err := unmarshalOTLPJSON(body, req); err != nil {
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
		if err := unmarshalOTLPJSON(body, req); err != nil {
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
