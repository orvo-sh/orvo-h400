package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/traceservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type TraceHandler struct {
	traceService traceservice.Service
	authService  authservice.Service
}

func NewTraceHandler(traceService traceservice.Service, authService authservice.Service) *TraceHandler {
	return &TraceHandler{
		traceService: traceService,
		authService:  authService,
	}
}

func (h *TraceHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "query-traces",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/traces",
		Tags:        []string{"traces"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.queryTraces)

	huma.Register(api, huma.Operation{
		OperationID: "get-trace",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/traces/{trace_id}",
		Tags:        []string{"traces"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getTrace)

	huma.Register(api, huma.Operation{
		OperationID: "get-trace-services",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/traces/services",
		Tags:        []string{"traces"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getServices)
}

func (h *TraceHandler) queryTraces(ctx context.Context, input *dto.QueryTracesInput) (*dto.QueryTracesOutput, error) {
	svcInput := traceservice.QueryTracesInput{
		OrganizationID: input.OrganizationID,
	}

	if input.Start != "" {
		if t, err := time.Parse(time.RFC3339, input.Start); err == nil {
			svcInput.StartTime = t
		}
	}
	if input.End != "" {
		if t, err := time.Parse(time.RFC3339, input.End); err == nil {
			svcInput.EndTime = t
		}
	}
	if input.Limit > 0 {
		svcInput.Limit = input.Limit
	}
	if input.Cursor != "" {
		if t, err := time.Parse(time.RFC3339Nano, input.Cursor); err == nil {
			svcInput.Cursor = &t
		}
	}
	if input.Search != "" {
		svcInput.SearchQuery = input.Search
	}
	if input.MinDuration > 0 {
		svcInput.MinDurationMs = input.MinDuration
	}
	if input.MaxDuration > 0 {
		svcInput.MaxDurationMs = input.MaxDuration
	}

	// Convenience filters: service and status become Filter entries
	if input.Service != "" {
		svcInput.Filters = append(svcInput.Filters, traceservice.Filter{
			Field:    "service",
			Operator: traceservice.FilterOperatorIn,
			Value:    input.Service,
		})
	}
	if input.Status != "" {
		// Convert status text values (ok, error, unset) to numeric codes
		statusValues := strings.Split(input.Status, ",")
		var codes []string
		for _, s := range statusValues {
			switch strings.TrimSpace(strings.ToLower(s)) {
			case "unset":
				codes = append(codes, "0")
			case "ok":
				codes = append(codes, "1")
			case "error":
				codes = append(codes, "2")
			}
		}
		if len(codes) > 0 {
			svcInput.Filters = append(svcInput.Filters, traceservice.Filter{
				Field:    "status_code",
				Operator: traceservice.FilterOperatorIn,
				Value:    strings.Join(codes, ","),
			})
		}
	}

	result, err := h.traceService.QueryTraces(ctx, svcInput)
	if err != nil {
		return nil, err
	}

	return &dto.QueryTracesOutput{
		Body: struct {
			Traces     []models.TraceSummary `json:"traces"`
			NextCursor *time.Time            `json:"next_cursor,omitempty"`
		}{
			Traces:     result.Traces,
			NextCursor: result.NextCursor,
		},
	}, nil
}

func (h *TraceHandler) getTrace(ctx context.Context, input *dto.GetTraceInput) (*dto.GetTraceOutput, error) {
	result, err := h.traceService.GetTrace(ctx, input.OrganizationID, input.TraceID)
	if err != nil {
		return nil, err
	}

	return &dto.GetTraceOutput{
		Body: struct {
			Spans []models.Span `json:"spans"`
		}{
			Spans: result.Spans,
		},
	}, nil
}

func (h *TraceHandler) getServices(ctx context.Context, input *dto.GetTraceServicesInput) (*dto.GetTraceServicesOutput, error) {
	services, err := h.traceService.GetServices(ctx, input.OrganizationID)
	if err != nil {
		return nil, err
	}

	return &dto.GetTraceServicesOutput{
		Body: struct {
			Services []string `json:"services"`
		}{
			Services: services,
		},
	}, nil
}
