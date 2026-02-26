package githubservice

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/providers/githubprovider"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

type Service interface {
	CreateInstallURL(ctx context.Context, input CreateInstallURLInput) (*CreateInstallURLOutput, apperr.Error)
	HandleSetupCallback(ctx context.Context, input HandleSetupCallbackInput) (*HandleSetupCallbackOutput, apperr.Error)
	HandleWebhook(ctx context.Context, input HandleWebhookInput) apperr.Error
	ListInstallations(ctx context.Context, organizationID string) ([]models.GithubInstallation, apperr.Error)
	ListRepositories(ctx context.Context, organizationID string) ([]models.GithubRepository, apperr.Error)
	SetRepositoryEnabled(ctx context.Context, input SetRepositoryEnabledInput) apperr.Error
	GetAutomationRepository(ctx context.Context, organizationID string, repositoryID string) (*AutomationRepository, apperr.Error)
}

type Config struct {
	SetupRedirectURL string
	StateTTL         time.Duration
	StateSecret      string
}

type CreateInstallURLInput struct {
	OrganizationID string
	UserID         string
}

type CreateInstallURLOutput struct {
	URL string
}

type HandleSetupCallbackInput struct {
	InstallationID int64
	SetupAction    string
	State          string
}

type HandleSetupCallbackOutput struct {
	RedirectURL string
}

type HandleWebhookInput struct {
	Event           string
	SignatureHeader string
	Payload         []byte
}

type SetRepositoryEnabledInput struct {
	OrganizationID string
	RepositoryID   string
	Enabled        bool
}

type AutomationRepository struct {
	Repository           models.GithubRepository
	GithubInstallationID int64
}

type service struct {
	postgres *postgres.DB
	logger   *slog.Logger
	provider githubprovider.Provider
	config   Config
}

func New(postgres *postgres.DB, logger *slog.Logger, provider githubprovider.Provider, config Config) Service {
	if config.StateTTL == 0 {
		config.StateTTL = 10 * time.Minute
	}
	return &service{
		postgres: postgres,
		logger:   logger.With("module", "github_service"),
		provider: provider,
		config:   config,
	}
}

func (s *service) CreateInstallURL(ctx context.Context, input CreateInstallURLInput) (*CreateInstallURLOutput, apperr.Error) {
	if input.OrganizationID == "" || input.UserID == "" {
		return nil, errs.ErrBadRequest
	}

	if err := s.ensureOwnerOrAdmin(ctx, input.OrganizationID, input.UserID); err != nil {
		return nil, err
	}

	stateRaw := util.GenerateRandomString(24)
	state := s.signState(stateRaw)
	stateHash := hashState(stateRaw)

	if _, err := s.postgres.Pool().Exec(ctx, `
		INSERT INTO github_install_states (id, organization_id, user_id, state_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, util.GenerateID("ghs"), input.OrganizationID, input.UserID, stateHash, time.Now().UTC().Add(s.config.StateTTL)); err != nil {
		s.logger.ErrorContext(ctx, "failed to persist github setup state", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	installURL := s.provider.BuildInstallationURL(state)
	if strings.TrimSpace(installURL) == "" {
		return nil, errs.ErrGithubNotConfigured
	}

	return &CreateInstallURLOutput{
		URL: installURL,
	}, nil
}

func (s *service) HandleSetupCallback(ctx context.Context, input HandleSetupCallbackInput) (*HandleSetupCallbackOutput, apperr.Error) {
	redirectBase := strings.TrimSpace(s.config.SetupRedirectURL)
	if redirectBase == "" {
		redirectBase = "/settings"
	}

	fail := func(code string) (*HandleSetupCallbackOutput, apperr.Error) {
		u, _ := url.Parse(redirectBase)
		q := u.Query()
		q.Set("github", "error")
		q.Set("code", code)
		u.RawQuery = q.Encode()
		return &HandleSetupCallbackOutput{RedirectURL: u.String()}, nil
	}

	if !s.enabled() {
		return fail(errs.ErrGithubNotConfigured.Code())
	}

	if input.InstallationID == 0 {
		return fail(errs.ErrBadRequest.Code())
	}

	stateEntry, appErr := s.consumeState(ctx, input.State)
	if appErr != nil {
		return fail(appErr.Code())
	}

	if err := s.ensureOwnerOrAdmin(ctx, stateEntry.OrganizationID, stateEntry.UserID); err != nil {
		return fail(err.Code())
	}

	internalInstallationID, appErr := s.upsertInstallation(ctx, stateEntry.OrganizationID, stateEntry.UserID, input.InstallationID)
	if appErr != nil {
		return fail(appErr.Code())
	}

	if appErr := s.syncInstallationRepositories(ctx, stateEntry.OrganizationID, internalInstallationID, input.InstallationID); appErr != nil {
		return fail(appErr.Code())
	}

	u, _ := url.Parse(redirectBase)
	q := u.Query()
	q.Set("github", "connected")
	q.Set("installation_id", strconv.FormatInt(input.InstallationID, 10))
	u.RawQuery = q.Encode()

	return &HandleSetupCallbackOutput{
		RedirectURL: u.String(),
	}, nil
}

func (s *service) HandleWebhook(ctx context.Context, input HandleWebhookInput) apperr.Error {
	if !s.enabled() {
		return errs.ErrGithubNotConfigured
	}

	if !s.provider.ValidateWebhookSignature(input.Payload, input.SignatureHeader) {
		return errs.ErrGithubInvalidWebhook
	}

	switch input.Event {
	case "installation":
		var payload struct {
			Action       string `json:"action"`
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
		}
		if err := jsonUnmarshal(input.Payload, &payload); err != nil {
			return errs.ErrBadRequest
		}

		if payload.Installation.ID == 0 {
			return nil
		}

		switch payload.Action {
		case "deleted":
			if _, err := s.postgres.Pool().Exec(ctx, `
				UPDATE github_installations
				SET active = FALSE, updated_at = NOW()
				WHERE github_installation_id = $1
			`, payload.Installation.ID); err != nil {
				s.logger.ErrorContext(ctx, "failed to deactivate github installation", slog.Any("error", err))
				return errs.ErrInternal
			}
		case "created", "new_permissions_accepted", "suspend", "unsuspend":
			if appErr := s.syncKnownInstallation(ctx, payload.Installation.ID); appErr != nil {
				return appErr
			}
		}
	case "installation_repositories":
		var payload struct {
			Action       string `json:"action"`
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
		}
		if err := jsonUnmarshal(input.Payload, &payload); err != nil {
			return errs.ErrBadRequest
		}
		if payload.Installation.ID == 0 {
			return nil
		}

		if appErr := s.syncKnownInstallation(ctx, payload.Installation.ID); appErr != nil {
			return appErr
		}
	}

	return nil
}

func (s *service) ListInstallations(ctx context.Context, organizationID string) ([]models.GithubInstallation, apperr.Error) {
	rows, err := s.postgres.Pool().Query(ctx, `
		SELECT id, organization_id, github_installation_id, account_id, account_login, account_type, active, created_at, updated_at
		FROM github_installations
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list github installations", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	items := make([]models.GithubInstallation, 0)
	for rows.Next() {
		var item models.GithubInstallation
		if err := rows.Scan(
			&item.ID,
			&item.OrganizationID,
			&item.GithubInstallationID,
			&item.AccountID,
			&item.AccountLogin,
			&item.AccountType,
			&item.Active,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			s.logger.ErrorContext(ctx, "failed to scan github installation", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "failed to iterate github installations", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return items, nil
}

func (s *service) ListRepositories(ctx context.Context, organizationID string) ([]models.GithubRepository, apperr.Error) {
	s.syncOrganizationRepositories(ctx, organizationID)

	rows, err := s.postgres.Pool().Query(ctx, `
		SELECT id, organization_id, installation_id, github_repository_id, full_name, default_branch, private, archived, enabled, last_synced_at, created_at, updated_at
		FROM github_repositories
		WHERE organization_id = $1
		ORDER BY enabled DESC, full_name ASC
	`, organizationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list github repositories", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	items := make([]models.GithubRepository, 0)
	for rows.Next() {
		var item models.GithubRepository
		if err := rows.Scan(
			&item.ID,
			&item.OrganizationID,
			&item.InstallationID,
			&item.GithubRepositoryID,
			&item.FullName,
			&item.DefaultBranch,
			&item.Private,
			&item.Archived,
			&item.Enabled,
			&item.LastSyncedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			s.logger.ErrorContext(ctx, "failed to scan github repository", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		s.logger.ErrorContext(ctx, "failed to iterate github repositories", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return items, nil
}

func (s *service) SetRepositoryEnabled(ctx context.Context, input SetRepositoryEnabledInput) apperr.Error {
	tag, err := s.postgres.Pool().Exec(ctx, `
		UPDATE github_repositories
		SET enabled = $3, updated_at = NOW()
		WHERE organization_id = $1
		  AND id = $2
	`, input.OrganizationID, input.RepositoryID, input.Enabled)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to update github repository enabled flag", slog.Any("error", err))
		return errs.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrGithubRepositoryNotFound
	}

	return nil
}

func (s *service) GetAutomationRepository(ctx context.Context, organizationID string, repositoryID string) (*AutomationRepository, apperr.Error) {
	var item AutomationRepository
	err := s.postgres.Pool().QueryRow(ctx, `
		SELECT
			r.id,
			r.organization_id,
			r.installation_id,
			r.github_repository_id,
			r.full_name,
			r.default_branch,
			r.private,
			r.archived,
			r.enabled,
			r.last_synced_at,
			r.created_at,
			r.updated_at,
			i.github_installation_id
		FROM github_repositories r
		JOIN github_installations i ON i.id = r.installation_id
		WHERE r.organization_id = $1
		  AND r.id = $2
		  AND r.enabled = TRUE
		  AND i.active = TRUE
	`, organizationID, repositoryID).Scan(
		&item.Repository.ID,
		&item.Repository.OrganizationID,
		&item.Repository.InstallationID,
		&item.Repository.GithubRepositoryID,
		&item.Repository.FullName,
		&item.Repository.DefaultBranch,
		&item.Repository.Private,
		&item.Repository.Archived,
		&item.Repository.Enabled,
		&item.Repository.LastSyncedAt,
		&item.Repository.CreatedAt,
		&item.Repository.UpdatedAt,
		&item.GithubInstallationID,
	)
	if err == nil {
		return &item, nil
	}
	if err != pgx.ErrNoRows {
		s.logger.ErrorContext(ctx, "failed to resolve automation repository", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	var enabled bool
	err = s.postgres.Pool().QueryRow(ctx, `
		SELECT enabled
		FROM github_repositories
		WHERE organization_id = $1
		  AND id = $2
	`, organizationID, repositoryID).Scan(&enabled)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrGithubRepositoryNotFound
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to resolve repository enabled state", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	if !enabled {
		return nil, errs.ErrGithubRepositoryDisabled
	}

	return nil, errs.ErrGithubRepositoryNotFound
}

func (s *service) enabled() bool {
	return s.provider != nil && s.provider.Enabled()
}

func (s *service) ensureOwnerOrAdmin(ctx context.Context, organizationID string, userID string) apperr.Error {
	var role string
	err := s.postgres.Pool().QueryRow(ctx, `
		SELECT role
		FROM organization_members
		WHERE organization_id = $1
		  AND user_id = $2
	`, organizationID, userID).Scan(&role)
	if err == pgx.ErrNoRows {
		return errs.ErrNotOrganizationMember
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to resolve organization membership role", slog.Any("error", err))
		return errs.ErrInternal
	}

	switch role {
	case string(models.OrganizationMemberRoleOwner), string(models.OrganizationMemberRoleAdmin):
		return nil
	default:
		return errs.ErrForbidden
	}
}

type setupStateEntry struct {
	OrganizationID string
	UserID         string
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
}

func (s *service) consumeState(ctx context.Context, signedState string) (*setupStateEntry, apperr.Error) {
	rawState, valid := s.verifyState(signedState)
	if !valid {
		return nil, errs.ErrGithubInvalidState
	}

	stateHash := hashState(rawState)
	var entry setupStateEntry
	err := s.postgres.Pool().QueryRow(ctx, `
		SELECT organization_id, user_id, expires_at, consumed_at
		FROM github_install_states
		WHERE state_hash = $1
	`, stateHash).Scan(
		&entry.OrganizationID,
		&entry.UserID,
		&entry.ExpiresAt,
		&entry.ConsumedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrGithubInvalidState
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load github setup state", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	if entry.ConsumedAt != nil {
		return nil, errs.ErrGithubStateAlreadyUsed
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		return nil, errs.ErrGithubStateExpired
	}

	tag, err := s.postgres.Pool().Exec(ctx, `
		UPDATE github_install_states
		SET consumed_at = NOW()
		WHERE state_hash = $1
		  AND consumed_at IS NULL
	`, stateHash)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to consume github setup state", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return nil, errs.ErrGithubStateAlreadyUsed
	}

	return &entry, nil
}

func (s *service) upsertInstallation(ctx context.Context, organizationID string, userID string, githubInstallationID int64) (string, apperr.Error) {
	var existingOrgID string
	err := s.postgres.Pool().QueryRow(ctx, `
		SELECT organization_id
		FROM github_installations
		WHERE github_installation_id = $1
	`, githubInstallationID).Scan(&existingOrgID)
	if err != nil && err != pgx.ErrNoRows {
		s.logger.ErrorContext(ctx, "failed to check github installation ownership", slog.Any("error", err))
		return "", errs.ErrInternal
	}
	if err == nil && existingOrgID != organizationID {
		return "", errs.ErrGithubInstallConflict
	}

	installation, providerErr := s.provider.GetInstallation(ctx, githubInstallationID)
	if providerErr != nil {
		s.logger.ErrorContext(ctx, "failed to query github installation metadata", slog.Any("error", providerErr))
		return "", errs.ErrInternal
	}

	var internalInstallationID string
	err = s.postgres.Pool().QueryRow(ctx, `
		INSERT INTO github_installations (
			id,
			organization_id,
			github_installation_id,
			account_id,
			account_login,
			account_type,
			created_by_user_id,
			active,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,TRUE,NOW(),NOW()
		)
		ON CONFLICT (github_installation_id) DO UPDATE
		SET
			account_id = EXCLUDED.account_id,
			account_login = EXCLUDED.account_login,
			account_type = EXCLUDED.account_type,
			active = TRUE,
			updated_at = NOW()
		RETURNING id
	`, util.GenerateID("ghi"), organizationID, githubInstallationID, installation.Account.ID, installation.Account.Login, installation.Account.Type, userID).Scan(&internalInstallationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to upsert github installation", slog.Any("error", err))
		return "", errs.ErrInternal
	}

	return internalInstallationID, nil
}

func (s *service) syncInstallationRepositories(ctx context.Context, organizationID string, installationID string, githubInstallationID int64) apperr.Error {
	repos, err := s.provider.ListInstallationRepositories(ctx, githubInstallationID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to list installation repositories", slog.Any("error", err))
		return errs.ErrInternal
	}

	repoIDs := make([]int64, 0, len(repos))
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
		if _, err := s.postgres.Pool().Exec(ctx, `
			INSERT INTO github_repositories (
				id,
				organization_id,
				installation_id,
				github_repository_id,
				full_name,
				default_branch,
				private,
				archived,
				enabled,
				last_synced_at,
				created_at,
				updated_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,FALSE,NOW(),NOW(),NOW()
			)
			ON CONFLICT (organization_id, github_repository_id) DO UPDATE
			SET
				installation_id = EXCLUDED.installation_id,
				full_name = EXCLUDED.full_name,
				default_branch = EXCLUDED.default_branch,
				private = EXCLUDED.private,
				archived = EXCLUDED.archived,
				last_synced_at = NOW(),
				updated_at = NOW()
		`, util.GenerateID("ghr"), organizationID, installationID, repo.ID, repo.FullName, repo.DefaultBranch, repo.Private, repo.Archived); err != nil {
			s.logger.ErrorContext(ctx, "failed to upsert github repository",
				slog.Int64("github_repository_id", repo.ID),
				slog.Any("error", err),
			)
			return errs.ErrInternal
		}
	}

	if len(repoIDs) == 0 {
		if _, err := s.postgres.Pool().Exec(ctx, `
			DELETE FROM github_repositories
			WHERE installation_id = $1
		`, installationID); err != nil {
			s.logger.ErrorContext(ctx, "failed to delete stale github repositories", slog.Any("error", err))
			return errs.ErrInternal
		}
		return nil
	}

	if _, err := s.postgres.Pool().Exec(ctx, `
		DELETE FROM github_repositories
		WHERE installation_id = $1
		  AND NOT (github_repository_id = ANY($2::BIGINT[]))
	`, installationID, repoIDs); err != nil {
		s.logger.ErrorContext(ctx, "failed to prune stale github repositories", slog.Any("error", err))
		return errs.ErrInternal
	}

	return nil
}

func (s *service) syncKnownInstallation(ctx context.Context, githubInstallationID int64) apperr.Error {
	if !s.enabled() {
		return errs.ErrGithubNotConfigured
	}

	var installationID string
	var organizationID string
	err := s.postgres.Pool().QueryRow(ctx, `
		SELECT id, organization_id
		FROM github_installations
		WHERE github_installation_id = $1
	`, githubInstallationID).Scan(&installationID, &organizationID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to resolve known github installation", slog.Any("error", err))
		return errs.ErrInternal
	}

	return s.syncInstallationRepositories(ctx, organizationID, installationID, githubInstallationID)
}

func (s *service) syncOrganizationRepositories(ctx context.Context, organizationID string) {
	if !s.enabled() {
		return
	}

	rows, err := s.postgres.Pool().Query(ctx, `
		SELECT id, github_installation_id
		FROM github_installations
		WHERE organization_id = $1
		  AND active = TRUE
	`, organizationID)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to list github installations for org sync", slog.Any("error", err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var installationID string
		var githubInstallationID int64
		if err := rows.Scan(&installationID, &githubInstallationID); err != nil {
			s.logger.WarnContext(ctx, "failed to scan github installation for org sync", slog.Any("error", err))
			continue
		}

		if appErr := s.syncInstallationRepositories(ctx, organizationID, installationID, githubInstallationID); appErr != nil {
			s.logger.WarnContext(ctx, "failed to sync github repositories for installation",
				slog.String("organization_id", organizationID),
				slog.String("installation_id", installationID),
				slog.Int64("github_installation_id", githubInstallationID),
				slog.String("error_code", appErr.Code()),
			)
		}
	}

	if err := rows.Err(); err != nil {
		s.logger.WarnContext(ctx, "failed to iterate github installations for org sync", slog.Any("error", err))
	}
}

func (s *service) signState(raw string) string {
	secret := s.stateSecret()
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(raw))
	signature := hex.EncodeToString(mac.Sum(nil))
	return raw + "." + signature
}

func (s *service) verifyState(signedState string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(signedState), ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	raw := parts[0]
	receivedSig := strings.ToLower(parts[1])

	mac := hmac.New(sha256.New, s.stateSecret())
	mac.Write([]byte(raw))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(receivedSig), []byte(expectedSig)) {
		return "", false
	}

	return raw, true
}

func (s *service) stateSecret() []byte {
	if strings.TrimSpace(s.config.StateSecret) == "" {
		return []byte("orvo-github-state-secret")
	}
	return []byte(s.config.StateSecret)
}

func hashState(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func jsonUnmarshal(payload []byte, out any) error {
	return json.Unmarshal(payload, out)
}
