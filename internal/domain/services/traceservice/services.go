package traceservice

import (
	"context"
	"log/slog"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) GetServices(ctx context.Context, organizationID string) ([]string, apperr.Error) {
	s.logger.InfoContext(ctx, "GetServices: getting services from spans", slog.String("organization_id", organizationID))

	query := `SELECT DISTINCT service_name
		FROM spans
		WHERE organization_id = ?
		  AND start_time > now() - INTERVAL 24 HOUR
		ORDER BY service_name ASC`

	rows, err := s.ch.Query(ctx, query, organizationID)
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
