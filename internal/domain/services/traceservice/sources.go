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

	query := `SELECT
		service_name,
		count() AS span_count,
		countIf(status_code = 2) AS error_count,
		avg(duration_ns) AS avg_duration_ns,
		max(start_time) AS last_seen
	FROM spans
	WHERE organization_id = ?
	  AND start_time > now() - INTERVAL 24 HOUR
	  AND service_name != ''
	GROUP BY service_name
	ORDER BY span_count DESC`

	rows, err := s.ch.Query(ctx, query, organizationID)
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
