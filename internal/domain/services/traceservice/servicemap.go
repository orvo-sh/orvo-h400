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

	query := `WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.trace_id,
			t.span_id,
			t.parent_span_id,
			t.service_name,
			t.status_code,
			t.duration_ns,
			t.start_time
		FROM traces_hot t
		WHERE t.organization_id = $1
		  AND t.start_time > NOW() - INTERVAL '24 hours'

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.trace_id,
			r.span_id,
			r.parent_span_id,
			r.service_name,
			r.status_code,
			r.duration_ns,
			r.start_time
		FROM traces_restored r
		WHERE r.organization_id = $1
		  AND r.start_time > NOW() - INTERVAL '24 hours'
		  AND NOT EXISTS (
			SELECT 1
			FROM traces_hot h
			WHERE h.organization_id = r.organization_id
			  AND h.id = r.id
		  )
	)
	SELECT
		parent.service_name AS source,
		child.service_name AS target,
		count(*)::BIGINT AS request_count,
		count(*) FILTER (WHERE child.status_code = 2)::BIGINT AS error_count,
		avg(child.duration_ns)::DOUBLE PRECISION AS avg_duration_ns
	FROM unioned child
	INNER JOIN unioned parent
		ON child.trace_id = parent.trace_id
		AND child.parent_span_id = parent.span_id
		AND child.organization_id = parent.organization_id
	WHERE child.service_name != parent.service_name
	  AND parent.service_name != ''
	  AND child.service_name != ''
	GROUP BY source, target
	ORDER BY request_count DESC`

	rows, err := s.pg.Pool().Query(ctx, query, organizationID)
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
