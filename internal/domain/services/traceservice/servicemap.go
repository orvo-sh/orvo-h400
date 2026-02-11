package traceservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetServiceMap(ctx context.Context, organizationID string) ([]models.ServiceEdge, apperr.Error) {
	s.logger.InfoContext(ctx, "GetServiceMap: building service map", slog.String("organization_id", organizationID))

	// Self-join spans to find cross-service caller→callee edges from the last 24h.
	query := `SELECT
		parent.service_name AS source,
		child.service_name AS target,
		count() AS request_count,
		countIf(child.status_code = 2) AS error_count,
		avg(child.duration_ns) AS avg_duration_ns
	FROM spans AS child
	INNER JOIN spans AS parent
		ON child.trace_id = parent.trace_id
		AND child.parent_span_id = parent.span_id
		AND child.organization_id = parent.organization_id
	WHERE child.organization_id = ?
	  AND child.start_time > now() - INTERVAL 24 HOUR
	  AND child.service_name != parent.service_name
	  AND parent.service_name != ''
	  AND child.service_name != ''
	GROUP BY source, target
	ORDER BY request_count DESC`

	rows, err := s.ch.Query(ctx, query, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetServiceMap: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var edges []models.ServiceEdge
	for rows.Next() {
		var e models.ServiceEdge
		if err := rows.Scan(&e.Source, &e.Target, &e.RequestCount, &e.ErrorCount, &e.AvgDurationNs); err != nil {
			s.logger.ErrorContext(ctx, "GetServiceMap: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetServiceMap: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return edges, nil
}
