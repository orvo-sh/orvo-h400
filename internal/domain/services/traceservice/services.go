package traceservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetServices(ctx context.Context, organizationID string) ([]string, apperr.Error) {
	s.logger.InfoContext(ctx, "GetServices: getting services from spans", slog.String("organization_id", organizationID))

	query := `WITH unioned AS (
		SELECT
			t.id,
			t.organization_id,
			t.service_name,
			t.start_time
		FROM traces_hot t
		WHERE t.organization_id = $1
		  AND t.start_time > NOW() - INTERVAL '24 hours'

		UNION ALL

		SELECT
			r.id,
			r.organization_id,
			r.service_name,
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
	SELECT DISTINCT service_name
	FROM unioned
	WHERE service_name != ''
	ORDER BY service_name ASC`

	rows, err := s.pg.Pool().Query(ctx, query, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetServices: query failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			s.logger.ErrorContext(ctx, "GetServices: scan failed", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		services = append(services, name)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "GetServices: rows iteration error", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return services, nil
}
