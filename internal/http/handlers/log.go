package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/logservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/helpers"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type LogHandler struct {
	logService  logservice.Service
	authService authservice.Service
}

func NewLogHandler(logService logservice.Service, authService authservice.Service) *LogHandler {
	return &LogHandler{
		logService:  logService,
		authService: authService,
	}
}

func (h *LogHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "query-logs",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/logs",
		Tags:        []string{"logs"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.queryLogs)

	huma.Register(api, huma.Operation{
		OperationID: "get-log-histogram",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/logs/histogram",
		Tags:        []string{"logs"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getHistogram)

	huma.Register(api, huma.Operation{
		OperationID: "get-log-services",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/logs/services",
		Tags:        []string{"logs"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getServices)
}

func (h *LogHandler) queryLogs(ctx context.Context, input *dto.QueryLogsInput) (*dto.QueryLogsOutput, error) {
	svcInput := logservice.QueryLogsInput{
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

	// Convenience filters: service and severity become Filter entries
	if input.Service != "" {
		svcInput.Filters = append(svcInput.Filters, logservice.Filter{
			Field:    "service",
			Operator: logservice.FilterOperatorIn,
			Value:    input.Service,
		})
	}
	if input.Severity != "" {
		svcInput.Filters = append(svcInput.Filters, logservice.Filter{
			Field:    "severity_text",
			Operator: logservice.FilterOperatorIn,
			Value:    strings.ToUpper(input.Severity),
		})
	}

	result, err := h.logService.QueryLogs(ctx, svcInput)
	if err != nil {
		return nil, helpers.ToHTTPError(err)
	}

	return &dto.QueryLogsOutput{
		Body: struct {
			Logs       []models.LogRecord `json:"logs"`
			NextCursor *time.Time         `json:"next_cursor,omitempty"`
		}{
			Logs:       result.Logs,
			NextCursor: result.NextCursor,
		},
	}, nil
}

func (h *LogHandler) getHistogram(ctx context.Context, input *dto.GetHistogramInput) (*dto.GetHistogramOutput, error) {
	svcInput := logservice.GetHistogramInput{
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
	if input.Interval != "" {
		svcInput.Interval = input.Interval
	}
	if input.Search != "" {
		svcInput.SearchQuery = input.Search
	}

	if input.Service != "" {
		svcInput.Filters = append(svcInput.Filters, logservice.Filter{
			Field:    "service",
			Operator: logservice.FilterOperatorIn,
			Value:    input.Service,
		})
	}
	if input.Severity != "" {
		svcInput.Filters = append(svcInput.Filters, logservice.Filter{
			Field:    "severity_text",
			Operator: logservice.FilterOperatorIn,
			Value:    strings.ToUpper(input.Severity),
		})
	}

	result, err := h.logService.GetHistogram(ctx, svcInput)
	if err != nil {
		return nil, helpers.ToHTTPError(err)
	}

	buckets := make([]dto.HistogramBucket, len(result.Buckets))
	for i, b := range result.Buckets {
		buckets[i] = dto.HistogramBucket{
			Time:  b.Time,
			Count: b.Count,
			Debug: b.Debug,
			Info:  b.Info,
			Warn:  b.Warn,
			Error: b.Error,
			Fatal: b.Fatal,
		}
	}

	return &dto.GetHistogramOutput{
		Body: struct {
			Buckets []dto.HistogramBucket `json:"buckets"`
		}{
			Buckets: buckets,
		},
	}, nil
}

func (h *LogHandler) getServices(ctx context.Context, input *dto.GetServicesInput) (*dto.GetServicesOutput, error) {
	services, err := h.logService.GetServices(ctx, input.OrganizationID)
	if err != nil {
		return nil, helpers.ToHTTPError(err)
	}

	return &dto.GetServicesOutput{
		Body: struct {
			Services []string `json:"services"`
		}{
			Services: services,
		},
	}, nil
}
