package traceservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetSources(ctx context.Context, organizationID string) ([]models.ServiceSource, apperr.Error) {
	s.logger.InfoContext(ctx, "GetSources: getting trace sources", slog.String("organization_id", organizationID))

	query := `WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
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
		service_name,
		count(*)::BIGINT AS span_count,
		count(*) FILTER (WHERE status_code = 2)::BIGINT AS error_count,
		avg(duration_ns)::DOUBLE PRECISION AS avg_duration_ns,
		max(start_time) AS last_seen
	FROM unioned
	WHERE service_name != ''
	GROUP BY service_name
	ORDER BY span_count DESC`

	rows, err := s.pg.Pool().Query(ctx, query, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetSources: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var sources []models.ServiceSource
	for rows.Next() {
		var src models.ServiceSource
		if err := rows.Scan(&src.ServiceName, &src.SpanCount, &src.ErrorCount, &src.AvgDurationNs, &src.LastSeen); err != nil {
			s.logger.ErrorContext(ctx, "GetSources: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		sources = append(sources, src)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetSources: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return sources, nil
}
