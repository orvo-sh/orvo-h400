package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetMetricCatalog(ctx context.Context, input GetMetricCatalogInput) (*GetMetricCatalogOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetMetricCatalog",
		slog.String("organization_id", input.OrganizationID),
		slog.String("service_name", input.ServiceName),
	)

	var (
		clauses []string
		args    []any
	)

	clauses = append(clauses, "organization_id = ?")
	args = append(args, input.OrganizationID)

	if input.ServiceName != "" {
		clauses = append(clauses, "service_name = ?")
		args = append(args, input.ServiceName)
	}

	if input.SearchQuery != "" {
		clauses = append(clauses, "metric_name ILIKE ?")
		args = append(args, "%"+input.SearchQuery+"%")
	}

	where := strings.Join(clauses, " AND ")

	query := fmt.Sprintf(`SELECT
		metric_name,
		any(metric_type) AS metric_type,
		any(metric_unit) AS metric_unit,
		any(metric_description) AS metric_description,
		any(service_name) AS service_name
	FROM metrics
	WHERE %s
	  AND time > now() - INTERVAL 24 HOUR
	GROUP BY metric_name
	ORDER BY metric_name ASC`, where)

	rows, err := s.ch.Query(ctx, query, args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetMetricCatalog: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var metrics []models.MetricMeta
	for rows.Next() {
		var m models.MetricMeta
		if err := rows.Scan(&m.Name, &m.Type, &m.Unit, &m.Description, &m.ServiceName); err != nil {
			s.logger.ErrorContext(ctx, "GetMetricCatalog: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetMetricCatalog: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &GetMetricCatalogOutput{Metrics: metrics}, nil
}
