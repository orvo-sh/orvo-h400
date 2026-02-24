package metricservice

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetMetricCatalog(ctx context.Context, input GetMetricCatalogInput) (*GetMetricCatalogOutput, apperr.Error) {
	s.logger.InfoContext(ctx, "GetMetricCatalog",
		slog.String("organization_id", input.OrganizationID),
		slog.String("service_name", input.ServiceName),
	)

	args := &metricSQLBuilder{}
	cutoff := time.Now().Add(-24 * time.Hour)

	hotWhere := []string{
		fmt.Sprintf("m.organization_id = %s", args.add(input.OrganizationID)),
		fmt.Sprintf("m.time >= %s", args.add(cutoff)),
	}
	restoredWhere := []string{
		fmt.Sprintf("r.organization_id = %s", args.add(input.OrganizationID)),
		fmt.Sprintf("r.time >= %s", args.add(cutoff)),
	}

	if input.ServiceName != "" {
		hotWhere = append(hotWhere, fmt.Sprintf("m.service_name = %s", args.add(input.ServiceName)))
		restoredWhere = append(restoredWhere, fmt.Sprintf("r.service_name = %s", args.add(input.ServiceName)))
	}
	if input.SearchQuery != "" {
		search := "%" + input.SearchQuery + "%"
		hotWhere = append(hotWhere, fmt.Sprintf("m.metric_name ILIKE %s", args.add(search)))
		restoredWhere = append(restoredWhere, fmt.Sprintf("r.metric_name ILIKE %s", args.add(search)))
	}

	query := fmt.Sprintf(`WITH unioned AS (
		SELECT
			m.id,
			m.organization_id,
			m.metric_name,
			m.metric_type,
			m.metric_unit,
			m.metric_description,
			m.service_name,
			m.time
		FROM metrics_hot m
		WHERE %s

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.metric_name,
			r.metric_type,
			r.metric_unit,
			r.metric_description,
			r.service_name,
			r.time
		FROM metrics_restored r
		WHERE %s
		  AND NOT EXISTS (
			SELECT 1
			FROM metrics_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
		metric_name,
		min(metric_type)::INT AS metric_type,
		min(metric_unit) AS metric_unit,
		min(metric_description) AS metric_description,
		min(service_name) AS service_name
	FROM unioned
	GROUP BY metric_name
	ORDER BY metric_name ASC`, strings.Join(hotWhere, " AND "), strings.Join(restoredWhere, " AND "))

	rows, err := s.pg.Pool().Query(ctx, query, args.args...)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetMetricCatalog: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var metrics []models.MetricMeta
	for rows.Next() {
		var m models.MetricMeta
		var metricType int
		if err := rows.Scan(&m.Name, &metricType, &m.Unit, &m.Description, &m.ServiceName); err != nil {
			s.logger.ErrorContext(ctx, "GetMetricCatalog: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		m.Type = metricTypeToString(metricType)
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetMetricCatalog: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &GetMetricCatalogOutput{Metrics: metrics}, nil
}

func metricTypeToString(metricType int) string {
	switch metricType {
	case 1:
		return "sum"
	case 2:
		return "gauge"
	case 3:
		return "histogram"
	default:
		return "unknown"
	}
}
