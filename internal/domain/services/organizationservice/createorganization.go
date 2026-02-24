package organizationservice

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode"

	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/internal/infra/postgres/db"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/pgutil"
	"github.com/orvo-sh/orvo/pkg/util"
)

type CreateOrganizationInput struct {
	Name          string
	Logo          *string
	CreatorUserID string
}

const maxSlugAttempts = 1024

func (s *service) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (*models.Organization, apperr.Error) {
	s.logger.InfoContext(ctx, "CreateOrganization: creating organization", slog.Any("input", input))

	var organization models.Organization
	err := s.postgres.WithTx(ctx, func(q *postgres.Queries) error {
		slug, err := generateUniqueSlug(ctx, q, input.Name)
		if err != nil {
			s.logger.Error("CreateOrganization: failed to generate slug", "error", err)
			return errs.ErrInternal
		}

		dbOrganization, err := q.CreateOrganization(ctx, db.CreateOrganizationParams{
			ID:       util.GenerateID("org"),
			Name:     input.Name,
			Slug:     slug,
			Logo:     pgutil.Text(input.Logo),
			Metadata: []byte("{}"),
		})
		if err != nil {
			s.logger.Error("CreateOrganization: failed to create organization", "error", err)
			return errs.ErrInternal
		}
		if _, err = q.CreateOrganizationMember(ctx, db.CreateOrganizationMemberParams{
			ID:             util.GenerateID("mem"),
			OrganizationID: dbOrganization.ID,
			UserID:         input.CreatorUserID,
			Role:           string(models.OrganizationMemberRoleOwner),
		}); err != nil {
			s.logger.Error("CreateOrganization: failed to create organization member", slog.Any("error", err))
			return errs.ErrInternal
		}

		organization = models.Organization{
			ID:        dbOrganization.ID,
			Name:      dbOrganization.Name,
			Logo:      pgutil.TextToPtr(dbOrganization.Logo),
			CreatedAt: dbOrganization.CreatedAt.Time,
			UpdatedAt: dbOrganization.UpdatedAt.Time,
		}
		return nil
	})

	if err != nil {
		if appErr, ok := err.(apperr.Error); ok {
			return nil, appErr
		}
		s.logger.ErrorContext(ctx, "Register: transaction failed", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &organization, nil
}

func generateUniqueSlug(ctx context.Context, q *postgres.Queries, name string) (string, error) {
	base := normalizeSlug(name)
	if base == "" {
		base = "organization"
	}

	candidate := base
	for i := 1; i <= maxSlugAttempts; i++ {
		exists, err := q.CheckSlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}

		suffix := strconv.Itoa(i + 1)
		maxBaseLen := 255 - len(suffix) - 1
		if maxBaseLen < 1 {
			maxBaseLen = 1
		}
		if len(base) > maxBaseLen {
			base = strings.Trim(base[:maxBaseLen], "-")
			if base == "" {
				base = "organization"
			}
		}
		candidate = fmt.Sprintf("%s-%s", base, suffix)
	}

	return "", fmt.Errorf("failed to allocate unique slug for %q after %d attempts", base, maxSlugAttempts)
}

func normalizeSlug(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(lower))
	prevDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsMark(r) {
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 255 {
		slug = strings.Trim(slug[:255], "-")
	}
	return slug
}
