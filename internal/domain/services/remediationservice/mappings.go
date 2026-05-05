package remediationservice

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

var serviceNameNormalizer = regexp.MustCompile(`[^a-z0-9]+`)

func (s *service) ListMappings(ctx context.Context, organizationID string) ([]models.ServiceRemediationMapping, apperr.Error) {
	rows, err := s.pg.Pool().Query(ctx, `
		SELECT
			m.id,
			m.organization_id,
			m.service_name,
			m.repository_id,
			r.full_name,
			m.created_by_user_id,
			m.updated_by_user_id,
			m.created_at,
			m.updated_at
		FROM service_remediation_mappings m
		JOIN github_repositories r ON r.id = m.repository_id
		WHERE m.organization_id = $1
		ORDER BY LOWER(m.service_name) ASC
	`, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list service remediation mappings", "error", err)
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	out := make([]models.ServiceRemediationMapping, 0)
	for rows.Next() {
		var item models.ServiceRemediationMapping
		if scanErr := rows.Scan(
			&item.ID,
			&item.OrganizationID,
			&item.ServiceName,
			&item.RepositoryID,
			&item.RepositoryFullName,
			&item.CreatedByUserID,
			&item.UpdatedByUserID,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			s.logger.ErrorContext(ctx, "failed to scan service remediation mapping", "error", scanErr)
			return nil, errs.ErrInternal
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "failed to iterate service remediation mappings", "error", err)
		return nil, errs.ErrInternal
	}

	return out, nil
}

func (s *service) UpsertMapping(ctx context.Context, input UpsertMappingInput) (*models.ServiceRemediationMapping, apperr.Error) {
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.RepositoryID = strings.TrimSpace(input.RepositoryID)
	input.ServiceName = strings.TrimSpace(input.ServiceName)
	input.UserID = strings.TrimSpace(input.UserID)

	if input.OrganizationID == "" || input.RepositoryID == "" || input.ServiceName == "" || input.UserID == "" {
		return nil, errs.ErrBadRequest
	}

	if _, appErr := s.githubService.GetAutomationRepository(ctx, input.OrganizationID, input.RepositoryID); appErr != nil {
		return nil, appErr
	}

	_, err := s.pg.Pool().Exec(ctx, `
		INSERT INTO service_remediation_mappings (
			id,
			organization_id,
			service_name,
			repository_id,
			created_by_user_id,
			updated_by_user_id,
			created_at,
			updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
		ON CONFLICT (organization_id, LOWER(service_name))
		DO UPDATE SET
			service_name = EXCLUDED.service_name,
			repository_id = EXCLUDED.repository_id,
			updated_by_user_id = EXCLUDED.updated_by_user_id,
			updated_at = NOW()
	`, util.GenerateID("srm"), input.OrganizationID, input.ServiceName, input.RepositoryID, input.UserID, input.UserID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert service remediation mapping", "error", err)
		return nil, errs.ErrInternal
	}

	return s.loadMappingForService(ctx, input.OrganizationID, input.ServiceName)
}

func (s *service) DeleteMapping(ctx context.Context, organizationID string, serviceName string) apperr.Error {
	organizationID = strings.TrimSpace(organizationID)
	serviceName = strings.TrimSpace(serviceName)
	if organizationID == "" || serviceName == "" {
		return errs.ErrBadRequest
	}

	tag, err := s.pg.Pool().Exec(ctx, `
		DELETE FROM service_remediation_mappings
		WHERE organization_id = $1
		  AND LOWER(service_name) = LOWER($2)
	`, organizationID, serviceName)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to delete service remediation mapping", "error", err)
		return errs.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}

	return nil
}

func (s *service) loadMappingForService(ctx context.Context, organizationID string, serviceName string) (*models.ServiceRemediationMapping, apperr.Error) {
	item, err := s.findMappingForService(ctx, organizationID, serviceName)
	if err == nil && item != nil {
		return item, nil
	}
	if err == pgx.ErrNoRows || item == nil {
		return nil, &errs.AutoResolveMappingMissingError{
			ServiceName: serviceName,
		}
	}

	s.logger.ErrorContext(ctx, "failed to load service remediation mapping", "error", err)
	return nil, errs.ErrInternal
}

func (s *service) findMappingForService(ctx context.Context, organizationID string, serviceName string) (*models.ServiceRemediationMapping, error) {
	serviceName = strings.TrimSpace(serviceName)
	if strings.TrimSpace(organizationID) == "" || serviceName == "" {
		return nil, pgx.ErrNoRows
	}

	var item models.ServiceRemediationMapping
	normalizedServiceName := normalizeServiceKey(serviceName)
	err := s.pg.Pool().QueryRow(ctx, `
		SELECT
			m.id,
			m.organization_id,
			m.service_name,
			m.repository_id,
			r.full_name,
			m.created_by_user_id,
			m.updated_by_user_id,
			m.created_at,
			m.updated_at
		FROM service_remediation_mappings m
		JOIN github_repositories r ON r.id = m.repository_id
		JOIN github_installations i ON i.id = r.installation_id
		WHERE m.organization_id = $1
		  AND (
			LOWER(m.service_name) = LOWER($2)
			OR regexp_replace(LOWER(m.service_name), '[^a-z0-9]+', '-', 'g') = $3
		  )
		  AND r.enabled = TRUE
		  AND i.active = TRUE
		ORDER BY CASE WHEN LOWER(m.service_name) = LOWER($2) THEN 0 ELSE 1 END, LOWER(m.service_name) ASC
		LIMIT 1
	`, organizationID, serviceName, normalizedServiceName).Scan(
		&item.ID,
		&item.OrganizationID,
		&item.ServiceName,
		&item.RepositoryID,
		&item.RepositoryFullName,
		&item.CreatedByUserID,
		&item.UpdatedByUserID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func normalizeServiceKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = serviceNameNormalizer.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func relatedServiceNames(primaryService string, traceSpans []models.Span) []string {
	primaryKey := normalizeServiceKey(primaryService)
	seen := map[string]struct{}{}
	out := make([]string, 0)

	record := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		key := normalizeServiceKey(candidate)
		if key == "" || key == primaryKey {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}

	for _, span := range traceSpans {
		record(span.ServiceName)
		record(span.ResourceAttributes["service.name"])
		record(span.SpanAttributes["peer.service"])
	}

	return out
}

func (s *service) loadRelatedRepositoryContexts(
	ctx context.Context,
	organizationID string,
	primaryService string,
	traceSpans []models.Span,
	primaryRepositoryID string,
) []models.AutoResolveRepositoryContext {
	services := relatedServiceNames(primaryService, traceSpans)
	if len(services) == 0 {
		return nil
	}

	seenRepositories := map[string]struct{}{}
	if strings.TrimSpace(primaryRepositoryID) != "" {
		seenRepositories[primaryRepositoryID] = struct{}{}
	}

	out := make([]models.AutoResolveRepositoryContext, 0, len(services))
	for _, serviceName := range services {
		mapping, err := s.findMappingForService(ctx, organizationID, serviceName)
		if err != nil {
			if err == pgx.ErrNoRows {
				continue
			}
			s.logger.WarnContext(ctx,
				"failed to resolve related service mapping",
				"service_name", serviceName,
				"error", err,
			)
			continue
		}
		if _, exists := seenRepositories[mapping.RepositoryID]; exists {
			continue
		}
		seenRepositories[mapping.RepositoryID] = struct{}{}
		out = append(out, models.AutoResolveRepositoryContext{
			ServiceName:        mapping.ServiceName,
			RepositoryID:       mapping.RepositoryID,
			RepositoryFullName: mapping.RepositoryFullName,
			Reason:             fmt.Sprintf("observed in trace with %s", primaryService),
		})
	}

	return out
}
