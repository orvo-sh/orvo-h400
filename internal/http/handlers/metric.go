package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/orvo-sh/orvo/internal/domain/services/authservice"
	"github.com/orvo-sh/orvo/internal/domain/services/metricservice"
	"github.com/orvo-sh/orvo/internal/http/dto"
	"github.com/orvo-sh/orvo/internal/http/middleware/authmiddleware"
)

type MetricHandler struct {
	metricService metricservice.Service
	authService   authservice.Service
}

func NewMetricHandler(metricService metricservice.Service, authService authservice.Service) *MetricHandler {
	return &MetricHandler{
		metricService: metricService,
		authService:   authService,
	}
}

func (h *MetricHandler) RegisterRoutes(api huma.API) {
	authMiddleware := authmiddleware.New(api, h.authService)

	huma.Register(api, huma.Operation{
		OperationID: "get-metric-catalog",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/metrics/catalog",
		Tags:        []string{"metrics"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getMetricCatalog)

	huma.Register(api, huma.Operation{
		OperationID: "query-timeseries",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/metrics/timeseries",
		Tags:        []string{"metrics"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.queryTimeseries)

	huma.Register(api, huma.Operation{
		OperationID: "get-red-metrics",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/metrics/red",
		Tags:        []string{"metrics"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getREDMetrics)

	huma.Register(api, huma.Operation{
		OperationID: "get-metric-summary",
		Method:      http.MethodGet,
		Path:        "/organizations/{organization_id}/metrics/summary",
		Tags:        []string{"metrics"},
		Middlewares: huma.Middlewares{authMiddleware},
	}, h.getMetricSummary)
}

func (h *MetricHandler) getMetricCatalog(ctx context.Context, input *dto.GetMetricCatalogInput) (*dto.GetMetricCatalogOutput, error) {
	result, err := h.metricService.GetMetricCatalog(ctx, metricservice.GetMetricCatalogInput{
		OrganizationID: input.OrganizationID,
		ServiceName:    input.Service,
		SearchQuery:    input.Search,
	})
	if err != nil {
		return nil, err
	}

	out := &dto.GetMetricCatalogOutput{}
	out.Body.Metrics = result.Metrics
	return out, nil
}

func (h *MetricHandler) queryTimeseries(ctx context.Context, input *dto.QueryTimeseriesInput) (*dto.QueryTimeseriesOutput, error) {
	svcInput := metricservice.QueryTimeseriesInput{
		OrganizationID: input.OrganizationID,
		MetricName:     input.MetricName,
		Step:           input.Step,
		Aggregation:    input.Aggregation,
		ServiceName:    input.Service,
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

	if input.GroupBy != "" {
		svcInput.GroupBy = splitAndTrim(input.GroupBy)
	}

	if input.Filters != "" {
		svcInput.Filters = parseFilterPairs(input.Filters)
	}

	result, err := h.metricService.QueryTimeseries(ctx, svcInput)
	if err != nil {
		return nil, err
	}

	out := &dto.QueryTimeseriesOutput{}
	out.Body.Series = result.Series
	return out, nil
}

func (h *MetricHandler) getREDMetrics(ctx context.Context, input *dto.GetREDMetricsInput) (*dto.GetREDMetricsOutput, error) {
	svcInput := metricservice.GetREDMetricsInput{
		OrganizationID: input.OrganizationID,
		ServiceName:    input.Service,
		Step:           input.Step,
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

	// Default to last 1 hour if no time range specified.
	if svcInput.StartTime.IsZero() {
		svcInput.StartTime = time.Now().Add(-1 * time.Hour)
	}
	if svcInput.EndTime.IsZero() {
		svcInput.EndTime = time.Now()
	}

	result, err := h.metricService.GetREDMetrics(ctx, svcInput)
	if err != nil {
		return nil, err
	}

	return &dto.GetREDMetricsOutput{Body: *result}, nil
}

func (h *MetricHandler) getMetricSummary(ctx context.Context, input *dto.GetMetricSummaryInput) (*dto.GetMetricSummaryOutput, error) {
	svcInput := metricservice.GetMetricSummaryInput{
		OrganizationID: input.OrganizationID,
		MetricName:     input.MetricName,
		Aggregation:    input.Aggregation,
		ServiceName:    input.Service,
	}

	if input.Lookback != "" {
		if d, err := time.ParseDuration(input.Lookback); err == nil {
			svcInput.LookbackWindow = d
		}
	}

	if input.Filters != "" {
		svcInput.Filters = parseFilterPairs(input.Filters)
	}

	result, err := h.metricService.GetMetricSummary(ctx, svcInput)
	if err != nil {
		return nil, err
	}

	out := &dto.GetMetricSummaryOutput{}
	out.Body.Value = result.Value
	out.Body.Timestamp = result.Timestamp.Format(time.RFC3339)
	return out, nil
}

// splitAndTrim splits a comma-separated string and trims whitespace.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseFilterPairs parses "key=value,key2=value2" into a map.
func parseFilterPairs(s string) map[string]string {
	filters := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			filters[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return filters
}
