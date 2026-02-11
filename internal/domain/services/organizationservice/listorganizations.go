package organizationservice

import (
	"context"
	"net/http"

	"github.com/orvo-sh/orvo/pkg/apperr"
)

func (s *service) ListOrganizations(ctx context.Context, userID string) ([]ListOrganizationItem, apperr.Error) {
	// Access Queries field explicitly
	rows, err := s.postgres.Queries.GetUserOrganizationMemberships(ctx, userID)
	if err != nil {
		s.logger.Error("failed to list user organizations", "error", err)
		return nil, apperr.New(http.StatusInternalServerError, "failed to list organizations")
	}

	items := make([]ListOrganizationItem, len(rows))
	for i, row := range rows {
		var logo *string
		if row.OrgLogo.Valid {
			str := row.OrgLogo.String
			logo = &str
		}
		items[i] = ListOrganizationItem{
			ID:        row.OrganizationID,
			Name:      row.OrgName,
			Slug:      row.OrgSlug,
			Logo:      logo,
			Role:      row.Role,
			CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return items, nil
}
