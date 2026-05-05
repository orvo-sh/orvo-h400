package workers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/providers/githubprovider"
	"github.com/orvo-sh/orvo/internal/domain/providers/sandboxprovider"
	"github.com/orvo-sh/orvo/internal/domain/services/githubservice"
	"github.com/orvo-sh/orvo/pkg/apperr"
	"github.com/orvo-sh/orvo/pkg/util"
)

type SandboxManagerConfig struct {
	DefaultImage     string
	WorkingDir       string
	CPULimit         string
	MemoryLimit      string
	JobTimeout       time.Duration
	CommandTimeout   time.Duration
	OpencodeTimeout  time.Duration
	BootstrapTimeout time.Duration
	GitAuthorName    string
	GitAuthorEmail   string
	MaxJobsPerPass   int
}

type SandboxManager struct {
	logger          *slog.Logger
	pool            *pgxpool.Pool
	githubProvider  githubprovider.Provider
	githubService   githubservice.Service
	sandboxProvider sandboxprovider.Provider
	config          SandboxManagerConfig
	trigger         chan struct{}
}

type CreateSandboxJobInput struct {
	OrganizationID      string
	RepositoryID        string
	RequestedBy         string
	Mode                string
	BaseBranch          string
	TaskTitle           string
	CommitMessage       string
	Commands            []string
	DraftPR             bool
	IncidentContextJSON string
	IncidentPrompt      string
}

type GetSandboxJobLogsOutput struct {
	Logs       []models.SandboxJobLog
	NextCursor int64
}

type sandboxJobExecutionContext struct {
	Job                  models.SandboxJob
	Repository           models.GithubRepository
	GithubInstallationID int64
	Commands             []models.SandboxJobCommand
}

func NewSandboxManager(
	logger *slog.Logger,
	pool *pgxpool.Pool,
	githubProvider githubprovider.Provider,
	githubService githubservice.Service,
	sandboxProvider sandboxprovider.Provider,
	config SandboxManagerConfig,
) *SandboxManager {
	if config.JobTimeout <= 0 {
		config.JobTimeout = 45 * time.Minute
	}
	if config.CommandTimeout <= 0 {
		config.CommandTimeout = 10 * time.Minute
	}
	if config.OpencodeTimeout <= 0 {
		config.OpencodeTimeout = 8 * time.Minute
	}
	if config.BootstrapTimeout <= 0 {
		config.BootstrapTimeout = 2 * time.Minute
	}
	if strings.TrimSpace(config.GitAuthorName) == "" {
		config.GitAuthorName = "orvo-bot"
	}
	if strings.TrimSpace(config.GitAuthorEmail) == "" {
		config.GitAuthorEmail = "orvo-bot@users.noreply.github.com"
	}
	if config.MaxJobsPerPass <= 0 {
		config.MaxJobsPerPass = 2
	}

	return &SandboxManager{
		logger:          logger.With("module", "sandbox_manager"),
		pool:            pool,
		githubProvider:  githubProvider,
		githubService:   githubService,
		sandboxProvider: sandboxProvider,
		config:          config,
		trigger:         make(chan struct{}, 1),
	}
}

func (m *SandboxManager) Enabled() bool {
	if m.githubProvider == nil || !m.githubProvider.Enabled() {
		return false
	}
	if m.sandboxProvider == nil || !m.sandboxProvider.Enabled() {
		return false
	}
	return true
}

func (m *SandboxManager) TriggerChan() <-chan struct{} {
	return m.trigger
}

func (m *SandboxManager) Notify() {
	select {
	case m.trigger <- struct{}{}:
	default:
	}
}

func (m *SandboxManager) CreateJob(ctx context.Context, input CreateSandboxJobInput) (*models.SandboxJob, apperr.Error) {
	if !m.Enabled() {
		return nil, errs.ErrSandboxNotConfigured
	}
	if strings.TrimSpace(input.OrganizationID) == "" || strings.TrimSpace(input.RepositoryID) == "" || strings.TrimSpace(input.RequestedBy) == "" {
		return nil, errs.ErrBadRequest
	}
	if len(input.Commands) == 0 {
		return nil, errs.ErrBadRequest
	}
	if strings.TrimSpace(input.CommitMessage) == "" {
		return nil, errs.ErrBadRequest
	}
	commands := make([]string, 0, len(input.Commands))
	for _, command := range input.Commands {
		command = strings.TrimSpace(command)
		if command != "" {
			commands = append(commands, command)
		}
	}
	if len(commands) == 0 {
		return nil, errs.ErrBadRequest
	}
	mode := strings.TrimSpace(strings.ToLower(input.Mode))
	if mode == "" {
		mode = "manual"
	}
	switch mode {
	case "manual":
		input.IncidentContextJSON = ""
		input.IncidentPrompt = ""
	case "auto_resolve":
		if strings.TrimSpace(input.IncidentContextJSON) == "" || strings.TrimSpace(input.IncidentPrompt) == "" {
			return nil, errs.ErrBadRequest
		}
	default:
		return nil, errs.ErrBadRequest
	}

	repo, appErr := m.githubService.GetAutomationRepository(ctx, input.OrganizationID, input.RepositoryID)
	if appErr != nil {
		return nil, appErr
	}

	jobID := util.GenerateID("sbj")
	if strings.TrimSpace(input.BaseBranch) == "" {
		input.BaseBranch = repo.Repository.DefaultBranch
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to begin sandbox job tx", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback(context.Background())
		}
	}()

	_, err = tx.Exec(ctx, `
		INSERT INTO sandbox_jobs (
			id,
			organization_id,
			repository_id,
			requested_by_user_id,
			mode,
			state,
			task_title,
			commit_message,
			base_branch,
			incident_context_json,
			incident_prompt,
			draft_pr,
			created_at,
			updated_at
		) VALUES (
			$1,$2,$3,$4,$5,'queued',$6,$7,$8,$9,$10,$11,NOW(),NOW()
		)
	`, jobID, input.OrganizationID, input.RepositoryID, input.RequestedBy, mode, input.TaskTitle, input.CommitMessage, input.BaseBranch, input.IncidentContextJSON, input.IncidentPrompt, input.DraftPR)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to create sandbox job", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	for idx, command := range commands {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sandbox_job_commands (
				id,
				sandbox_job_id,
				ordinal,
				command,
				created_at,
				updated_at
			) VALUES (
				$1,$2,$3,$4,NOW(),NOW()
			)
		`, util.GenerateID("sbc"), jobID, idx+1, command); err != nil {
			m.logger.ErrorContext(ctx, "failed to create sandbox job command", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
	}

	if err := tx.Commit(ctx); err != nil {
		m.logger.ErrorContext(ctx, "failed to commit sandbox job tx", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	rollback = false

	m.Notify()
	return m.GetJob(ctx, input.OrganizationID, jobID)
}

func (m *SandboxManager) GetJob(ctx context.Context, organizationID string, jobID string) (*models.SandboxJob, apperr.Error) {
	job, err := m.loadJob(ctx, organizationID, jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrSandboxJobNotFound
		}
		m.logger.ErrorContext(ctx, "failed to load sandbox job", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	return job, nil
}

func (m *SandboxManager) GetLogs(ctx context.Context, organizationID string, jobID string, cursor int64, limit int) (*GetSandboxJobLogsOutput, apperr.Error) {
	if cursor < 0 {
		cursor = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	if !m.jobExists(ctx, organizationID, jobID) {
		return nil, errs.ErrSandboxJobNotFound
	}

	rows, err := m.pool.Query(ctx, `
		SELECT l.seq, l.stream, l.message, l.created_at
		FROM sandbox_job_logs l
		JOIN sandbox_jobs j ON j.id = l.sandbox_job_id
		WHERE j.organization_id = $1
		  AND l.sandbox_job_id = $2
		  AND l.seq > $3
		ORDER BY l.seq ASC
		LIMIT $4
	`, organizationID, jobID, cursor, limit)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to list sandbox job logs", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	logs := make([]models.SandboxJobLog, 0)
	nextCursor := cursor
	for rows.Next() {
		var item models.SandboxJobLog
		if err := rows.Scan(&item.Seq, &item.Stream, &item.Message, &item.CreatedAt); err != nil {
			m.logger.ErrorContext(ctx, "failed to scan sandbox job log", slog.Any("error", err))
			return nil, errs.ErrInternal
		}
		logs = append(logs, item)
		nextCursor = item.Seq
	}
	if err := rows.Err(); err != nil {
		m.logger.ErrorContext(ctx, "failed to iterate sandbox job logs", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return &GetSandboxJobLogsOutput{
		Logs:       logs,
		NextCursor: nextCursor,
	}, nil
}

func (m *SandboxManager) ListJobs(ctx context.Context, organizationID string, states []string, limit int) ([]models.SandboxJob, apperr.Error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, errs.ErrBadRequest
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	filterStates := make([]string, 0, len(states))
	for _, state := range states {
		trimmed := strings.TrimSpace(strings.ToLower(state))
		if trimmed == "" {
			continue
		}
		filterStates = append(filterStates, trimmed)
	}

	rows, err := m.pool.Query(ctx, `
		SELECT
			id,
			organization_id,
			repository_id,
			requested_by_user_id,
			mode,
			state,
			runtime_type,
			sandbox_instance_id,
			task_title,
			commit_message,
			base_branch,
			incident_context_json,
			incident_prompt,
			branch_name,
			draft_pr,
			pull_request_number,
			pull_request_url,
			cancel_requested,
			error,
			created_at,
			started_at,
			finished_at,
			updated_at
		FROM sandbox_jobs
		WHERE organization_id = $1
		  AND (
			COALESCE(array_length($2::TEXT[], 1), 0) = 0
			OR state = ANY($2::TEXT[])
		  )
		ORDER BY
			CASE state
				WHEN 'running' THEN 0
				WHEN 'queued' THEN 1
				ELSE 2
			END,
			created_at DESC
		LIMIT $3
	`, organizationID, filterStates, limit)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to list sandbox jobs", slog.Any("error", err))
		return nil, errs.ErrInternal
	}
	defer rows.Close()

	out := make([]models.SandboxJob, 0)
	for rows.Next() {
		var job models.SandboxJob
		var prNumber sql.NullInt64
		var prURL sql.NullString
		var startedAt sql.NullTime
		var finishedAt sql.NullTime
		if err := rows.Scan(
			&job.ID,
			&job.OrganizationID,
			&job.RepositoryID,
			&job.RequestedByUserID,
			&job.Mode,
			&job.State,
			&job.RuntimeType,
			&job.SandboxInstanceID,
			&job.TaskTitle,
			&job.CommitMessage,
			&job.BaseBranch,
			&job.IncidentContext,
			&job.IncidentPrompt,
			&job.BranchName,
			&job.DraftPR,
			&prNumber,
			&prURL,
			&job.CancelRequested,
			&job.Error,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&job.UpdatedAt,
		); err != nil {
			m.logger.ErrorContext(ctx, "failed to scan sandbox job", slog.Any("error", err))
			return nil, errs.ErrInternal
		}

		if prNumber.Valid {
			v := prNumber.Int64
			job.PullRequestNumber = &v
		}
		if prURL.Valid {
			v := prURL.String
			job.PullRequestURL = &v
		}
		if startedAt.Valid {
			v := startedAt.Time
			job.StartedAt = &v
		}
		if finishedAt.Valid {
			v := finishedAt.Time
			job.FinishedAt = &v
		}

		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		m.logger.ErrorContext(ctx, "failed iterating sandbox jobs", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	return out, nil
}

func (m *SandboxManager) CancelJob(ctx context.Context, organizationID string, jobID string) apperr.Error {
	tag, err := m.pool.Exec(ctx, `
		UPDATE sandbox_jobs
		SET
			cancel_requested = TRUE,
			state = CASE WHEN state = 'queued' THEN 'cancelled' ELSE state END,
			finished_at = CASE WHEN state = 'queued' THEN NOW() ELSE finished_at END,
			updated_at = NOW()
		WHERE organization_id = $1
		  AND id = $2
	`, organizationID, jobID)
	if err != nil {
		m.logger.ErrorContext(ctx, "failed to cancel sandbox job", slog.Any("error", err))
		return errs.ErrInternal
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrSandboxJobNotFound
	}

	m.appendLog(ctx, jobID, "system", "cancel requested")
	return nil
}

func (m *SandboxManager) ProcessQueued(ctx context.Context) error {
	if !m.Enabled() {
		return nil
	}

	for i := 0; i < m.config.MaxJobsPerPass; i++ {
		jobID, err := m.claimNextJob(ctx)
		if err != nil {
			return err
		}
		if jobID == "" {
			return nil
		}

		if err := m.processJob(ctx, jobID); err != nil {
			m.logger.ErrorContext(ctx, "sandbox job processing failed", slog.String("job_id", jobID), slog.Any("error", err))
		}
	}

	return nil
}

func (m *SandboxManager) processJob(ctx context.Context, jobID string) error {
	execCtx, cancel := context.WithTimeout(ctx, m.config.JobTimeout)
	defer cancel()

	jobCtx, err := m.loadJobExecutionContext(execCtx, jobID)
	if err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("load job context failed: %v", err))
	}
	m.appendLog(execCtx, jobID, "system", "job claimed by sandbox worker")

	sessionName := sanitizeContainerName("orvo-" + strings.ReplaceAll(jobID, "_", "-"))
	image := m.config.DefaultImage
	m.appendLog(execCtx, jobID, "system", fmt.Sprintf("creating sandbox session with image %s", image))
	startupTimeout := m.config.BootstrapTimeout
	if startupTimeout <= 0 || startupTimeout > 5*time.Minute {
		startupTimeout = 2 * time.Minute
	}
	startupCtx, startupCancel := context.WithTimeout(execCtx, startupTimeout)
	session, err := m.sandboxProvider.CreateSession(startupCtx, sandboxprovider.CreateSessionInput{
		Name:        sessionName,
		Image:       image,
		WorkingDir:  m.config.WorkingDir,
		CPULimit:    m.config.CPULimit,
		MemoryLimit: m.config.MemoryLimit,
	})
	startupCancel()
	if err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("sandbox start failed: %v", err))
	}
	m.appendLog(execCtx, jobID, "system", fmt.Sprintf("sandbox session ready: %s (%s)", session.ID, session.Runtime))
	defer func() {
		destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer destroyCancel()
		if err := m.sandboxProvider.Destroy(destroyCtx, session); err != nil {
			m.logger.Warn("failed to destroy sandbox session", slog.String("job_id", jobID), slog.Any("error", err))
		}
	}()

	if _, err := m.pool.Exec(execCtx, `
		UPDATE sandbox_jobs
		SET runtime_type = $2, sandbox_instance_id = $3, updated_at = NOW()
		WHERE id = $1
	`, jobID, string(session.Runtime), session.ID); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to update runtime metadata: %v", err))
	}

	installationToken, _, err := m.githubProvider.CreateInstallationToken(execCtx, jobCtx.GithubInstallationID)
	if err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to create installation token: %v", err))
	}

	baseBranch := strings.TrimSpace(jobCtx.Job.BaseBranch)
	if baseBranch == "" {
		baseBranch = jobCtx.Repository.DefaultBranch
	}
	if baseBranch == "" {
		baseBranch = "main"
	}

	branchName := buildBranchName(jobID)
	if _, err := m.pool.Exec(execCtx, `
		UPDATE sandbox_jobs
		SET branch_name = $2, updated_at = NOW()
		WHERE id = $1
	`, jobID, branchName); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to persist branch name: %v", err))
	}

	masks := []string{installationToken}
	workspaceDir := "repo"
	cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", installationToken, jobCtx.Repository.FullName)

	if err := m.runSystemCommandWithTimeout(execCtx, session, jobID, "if ! command -v git >/dev/null 2>&1; then if command -v apt-get >/dev/null 2>&1; then apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git ca-certificates; elif command -v apk >/dev/null 2>&1; then apk add --no-cache git ca-certificates; elif command -v dnf >/dev/null 2>&1; then dnf install -y git ca-certificates; elif command -v yum >/dev/null 2>&1; then yum install -y git ca-certificates; else exit 1; fi; fi", "ensure git is installed", masks, m.config.BootstrapTimeout); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "git config --global user.name "+shellQuote(m.config.GitAuthorName), "git setup user.name", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "git config --global user.email "+shellQuote(m.config.GitAuthorEmail), "git setup user.email", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "git clone "+shellQuote(cloneURL)+" "+shellQuote(workspaceDir), "clone repository", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "cd repo && git checkout "+shellQuote(baseBranch), "checkout base branch", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "cd repo && git pull --ff-only origin "+shellQuote(baseBranch), "pull base branch", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(execCtx, session, jobID, "cd repo && git checkout -b "+shellQuote(branchName), "create working branch", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}

	if strings.EqualFold(jobCtx.Job.Mode, "auto_resolve") {
		if err := m.prepareAutoResolveContext(execCtx, session, jobCtx); err != nil {
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
		}
		if err := m.prepareRelatedRepositories(execCtx, session, jobCtx, installationToken, masks); err != nil {
			m.appendLog(execCtx, jobID, "stderr", sanitizeLog(fmt.Sprintf("related repository context unavailable: %v", err), masks))
		}
	} else {
		_ = m.runSystemCommandAllowFailure(execCtx, session, jobID, "cd repo && if [ -f package.json ]; then if [ -f pnpm-lock.yaml ] && command -v pnpm >/dev/null 2>&1; then pnpm install --frozen-lockfile; elif [ -f package-lock.json ] && command -v npm >/dev/null 2>&1; then npm ci; elif command -v npm >/dev/null 2>&1; then npm install; fi; fi", "bootstrap node dependencies", masks)
		_ = m.runSystemCommandAllowFailure(execCtx, session, jobID, "cd repo && if [ -f go.mod ] && command -v go >/dev/null 2>&1; then go mod download; fi", "bootstrap go modules", masks)
	}

	for _, command := range jobCtx.Commands {
		if m.isCancelRequested(execCtx, jobID) {
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateCancelled), "job cancelled")
		}

		if err := m.markCommandStarted(execCtx, command.ID); err != nil {
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to mark command start: %v", err))
		}
		m.appendLog(execCtx, jobID, "system", fmt.Sprintf("command #%d started: %s", command.Ordinal, command.Command))

		result, runErr := m.execWithHeartbeat(
			execCtx,
			session,
			jobID,
			"cd repo && "+command.Command,
			fmt.Sprintf("command #%d", command.Ordinal),
			nil,
			m.commandTimeoutForJob(jobCtx.Job, command),
		)
		if runErr != nil {
			_ = m.markCommandFinished(execCtx, command.ID, 1)
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to execute command: %v", runErr))
		}

		_ = m.markCommandFinished(execCtx, command.ID, result.ExitCode)
		m.logCommandResult(execCtx, jobID, fmt.Sprintf("command #%d: %s", command.Ordinal, command.Command), result, masks)

		if result.TimedOut {
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateTimedOut), fmt.Sprintf("command timed out: %s", command.Command))
		}
		if result.ExitCode != 0 {
			return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("command failed with exit code %d: %s", result.ExitCode, command.Command))
		}
	}

	autoResolveSummary := ""
	if strings.EqualFold(jobCtx.Job.Mode, "auto_resolve") {
		autoResolveSummary = m.readAutoResolveSummary(execCtx, session)
		if autoResolveSummary == "" {
			autoResolveSummary = m.loadOpencodeSummaryFromLogs(execCtx, jobID)
		}
		if err := m.cleanupAutoResolveArtifacts(execCtx, session, jobID, masks); err != nil {
			m.appendLog(execCtx, jobID, "stderr", sanitizeLog(fmt.Sprintf("cleanup auto resolve artifacts failed: %v", err), masks))
		}
	}

	statusResult, err := m.sandboxProvider.Exec(execCtx, session, "cd repo && git status --porcelain", m.config.CommandTimeout)
	if err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to inspect git status: %v", err))
	}
	if strings.TrimSpace(statusResult.Stdout) == "" {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), "no changes produced by commands")
	}

	if err := m.runSystemCommand(execCtx, session, jobID, "cd repo && git add -A && git commit -m "+shellQuote(jobCtx.Job.CommitMessage), "commit changes", masks); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}
	if err := m.runSystemCommand(
		execCtx,
		session,
		jobID,
		"cd repo && rc=1; for attempt in 1 2 3; do git push -u origin "+shellQuote(branchName)+" && rc=0 && break; rc=$?; echo \"git push attempt ${attempt} failed\" >&2; if command -v getent >/dev/null 2>&1; then getent hosts github.com >&2 || true; elif command -v nslookup >/dev/null 2>&1; then nslookup github.com >&2 || true; fi; sleep $((attempt*3)); done; exit $rc",
		"push branch",
		masks,
	); err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), err.Error())
	}

	owner, repoName, parseErr := splitRepositoryFullName(jobCtx.Repository.FullName)
	if parseErr != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), parseErr.Error())
	}

	prTitle := strings.TrimSpace(jobCtx.Job.TaskTitle)
	if prTitle == "" {
		prTitle = jobCtx.Job.CommitMessage
	}
	prBody := m.buildPullRequestBody(execCtx, session, jobID, jobCtx, autoResolveSummary)

	pr, err := m.githubProvider.CreatePullRequest(execCtx, jobCtx.GithubInstallationID, githubprovider.CreatePullRequestInput{
		Owner: owner,
		Repo:  repoName,
		Title: prTitle,
		Body:  prBody,
		Head:  branchName,
		Base:  baseBranch,
		Draft: jobCtx.Job.DraftPR,
	})
	if err != nil {
		return m.failJob(execCtx, jobID, string(models.SandboxJobStateFailed), fmt.Sprintf("failed to create pull request: %v", err))
	}

	if _, err := m.pool.Exec(execCtx, `
		UPDATE sandbox_jobs
		SET
			state = 'succeeded',
			branch_name = $2,
			pull_request_number = $3,
			pull_request_url = $4,
			error = '',
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, jobID, branchName, pr.Number, pr.HTMLURL); err != nil {
		return err
	}

	m.appendLog(execCtx, jobID, "system", fmt.Sprintf("pull request created: %s", pr.HTMLURL))
	return nil
}

func (m *SandboxManager) runSystemCommand(ctx context.Context, session *sandboxprovider.Session, jobID string, command string, displayName string, masks []string) error {
	return m.runSystemCommandWithTimeout(ctx, session, jobID, command, displayName, masks, m.config.CommandTimeout)
}

func (m *SandboxManager) runSystemCommandWithTimeout(ctx context.Context, session *sandboxprovider.Session, jobID string, command string, displayName string, masks []string, timeout time.Duration) error {
	m.appendLog(ctx, jobID, "system", sanitizeLog(displayName+" (started)", masks))
	result, err := m.execWithHeartbeat(ctx, session, jobID, command, displayName, masks, timeout)
	if err != nil {
		return fmt.Errorf("%s failed: %w", displayName, err)
	}

	m.logCommandResult(ctx, jobID, displayName, result, masks)

	if result.TimedOut {
		return fmt.Errorf("%s timed out", displayName)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s failed with exit code %d", displayName, result.ExitCode)
	}
	return nil
}

func (m *SandboxManager) runSystemCommandAllowFailure(ctx context.Context, session *sandboxprovider.Session, jobID string, command string, displayName string, masks []string) error {
	m.appendLog(ctx, jobID, "system", sanitizeLog(displayName+" (started, best effort)", masks))
	result, err := m.execWithHeartbeat(ctx, session, jobID, command, displayName, masks, m.config.CommandTimeout)
	if err != nil {
		m.appendLog(ctx, jobID, "stderr", sanitizeLog(fmt.Sprintf("%s execution error: %v", displayName, err), masks))
		return err
	}
	m.logCommandResult(ctx, jobID, displayName+" (best effort)", result, masks)
	return nil
}

func (m *SandboxManager) execWithHeartbeat(
	ctx context.Context,
	session *sandboxprovider.Session,
	jobID string,
	command string,
	label string,
	masks []string,
	timeout time.Duration,
) (*sandboxprovider.ExecResult, error) {
	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.appendLog(ctx, jobID, "system", sanitizeLog(fmt.Sprintf("%s still running...", label), masks))
			}
		}
	}()

	return m.sandboxProvider.Exec(ctx, session, command, timeout)
}

func (m *SandboxManager) commandTimeoutForJob(job models.SandboxJob, command models.SandboxJobCommand) time.Duration {
	timeout := m.config.CommandTimeout
	if !strings.EqualFold(job.Mode, "auto_resolve") {
		return timeout
	}
	if !strings.Contains(command.Command, models.AutoResolveOpencodeCommandMarker) {
		return timeout
	}

	// Let opencode hit its own budget and flush logs before the generic worker timeout ends the process.
	opencodeBudget := m.config.OpencodeTimeout + (15 * time.Second)
	if opencodeBudget > timeout {
		return opencodeBudget
	}
	return timeout
}

func (m *SandboxManager) prepareAutoResolveContext(ctx context.Context, session *sandboxprovider.Session, jobCtx *sandboxJobExecutionContext) error {
	contextJSON := strings.TrimSpace(jobCtx.Job.IncidentContext)
	prompt := strings.TrimSpace(jobCtx.Job.IncidentPrompt)
	if contextJSON == "" || prompt == "" {
		return fmt.Errorf("auto resolve context is missing")
	}

	contextB64 := base64.StdEncoding.EncodeToString([]byte(contextJSON))
	promptB64 := base64.StdEncoding.EncodeToString([]byte(prompt))
	masks := []string{contextB64, promptB64}

	if err := m.runSystemCommand(ctx, session, jobCtx.Job.ID, "cd repo && mkdir -p .orvo", "prepare .orvo workspace", masks); err != nil {
		return err
	}
	if err := m.runSystemCommand(
		ctx,
		session,
		jobCtx.Job.ID,
		"cd repo && printf '%s' "+shellQuote(contextB64)+" | base64 -d > .orvo/incident-context.json",
		"write incident context",
		masks,
	); err != nil {
		return err
	}
	if err := m.runSystemCommand(
		ctx,
		session,
		jobCtx.Job.ID,
		"cd repo && printf '%s' "+shellQuote(promptB64)+" | base64 -d > .orvo/opencode-prompt.txt",
		"write opencode prompt",
		masks,
	); err != nil {
		return err
	}

	return nil
}

func (m *SandboxManager) prepareRelatedRepositories(
	ctx context.Context,
	session *sandboxprovider.Session,
	jobCtx *sandboxJobExecutionContext,
	installationToken string,
	masks []string,
) error {
	incident := parseAutoResolveIncidentContext(jobCtx.Job.IncidentContext)
	if incident == nil || len(incident.RelatedRepositories) == 0 {
		return nil
	}

	type relatedRepositoryWorkspace struct {
		ServiceName        string `json:"service_name"`
		RepositoryFullName string `json:"repository_full_name"`
		Path               string `json:"path"`
		Reason             string `json:"reason"`
	}

	entries := make([]relatedRepositoryWorkspace, 0, len(incident.RelatedRepositories))
	for _, repo := range incident.RelatedRepositories {
		repositoryFullName := strings.TrimSpace(repo.RepositoryFullName)
		if repositoryFullName == "" {
			continue
		}
		dirName := sanitizeRelatedRepoDir(repositoryFullName)
		targetPath := "/workspace/related/" + dirName
		cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", installationToken, repositoryFullName)

		if err := m.runSystemCommand(
			ctx,
			session,
			jobCtx.Job.ID,
			"mkdir -p /workspace/related && rm -rf "+shellQuote(targetPath)+" && git clone --depth 1 "+shellQuote(cloneURL)+" "+shellQuote(targetPath)+" && chmod -R a-w "+shellQuote(targetPath),
			"clone related repository "+repositoryFullName,
			append(masks, cloneURL),
		); err != nil {
			return err
		}

		entries = append(entries, relatedRepositoryWorkspace{
			ServiceName:        repo.ServiceName,
			RepositoryFullName: repositoryFullName,
			Path:               targetPath,
			Reason:             repo.Reason,
		})
	}

	if len(entries) == 0 {
		return nil
	}

	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	manifestB64 := base64.StdEncoding.EncodeToString(raw)
	return m.runSystemCommand(
		ctx,
		session,
		jobCtx.Job.ID,
		"cd repo && printf '%s' "+shellQuote(manifestB64)+" | base64 -d > .orvo/related-repositories.json",
		"write related repository context",
		append(masks, manifestB64),
	)
}

func (m *SandboxManager) readAutoResolveSummary(ctx context.Context, session *sandboxprovider.Session) string {
	result, err := m.sandboxProvider.Exec(ctx, session, "cd repo && [ -f .orvo/auto-resolve-summary.md ] && cat .orvo/auto-resolve-summary.md", 10*time.Second)
	if err != nil || result == nil || result.ExitCode != 0 {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

func (m *SandboxManager) cleanupAutoResolveArtifacts(
	ctx context.Context,
	session *sandboxprovider.Session,
	jobID string,
	masks []string,
) error {
	return m.runSystemCommandAllowFailure(
		ctx,
		session,
		jobID,
		"cd repo && rm -rf .orvo",
		"remove auto resolve artifacts",
		masks,
	)
}

func sanitizeRelatedRepoDir(repositoryFullName string) string {
	repositoryFullName = strings.TrimSpace(repositoryFullName)
	if repositoryFullName == "" {
		return "repo"
	}
	repositoryFullName = strings.ReplaceAll(repositoryFullName, "/", "__")
	repositoryFullName = strings.ReplaceAll(repositoryFullName, " ", "-")
	return repositoryFullName
}

func (m *SandboxManager) loadOpencodeSummaryFromLogs(ctx context.Context, jobID string) string {
	rows, err := m.pool.Query(ctx, `
		SELECT message
		FROM sandbox_job_logs
		WHERE sandbox_job_id = $1
		  AND stream = 'stdout'
		ORDER BY seq ASC
	`, jobID)
	if err != nil {
		return ""
	}
	defer rows.Close()

	summaries := make([]string, 0, 2)
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return ""
		}
		text := extractOpencodeTextEvent(message)
		if text == "" {
			continue
		}
		summaries = append(summaries, text)
		if len(summaries) > 2 {
			summaries = summaries[len(summaries)-2:]
		}
	}
	if err := rows.Err(); err != nil || len(summaries) == 0 {
		return ""
	}

	return strings.Join(summaries, "\n\n")
}

func extractOpencodeTextEvent(message string) string {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "{") {
		return ""
	}

	var payload struct {
		Type string `json:"type"`
		Part struct {
			Text string `json:"text"`
		} `json:"part"`
	}
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return ""
	}
	if payload.Type != "text" {
		return ""
	}
	return strings.TrimSpace(payload.Part.Text)
}

func (m *SandboxManager) logCommandResult(ctx context.Context, jobID string, label string, result *sandboxprovider.ExecResult, masks []string) {
	m.appendLog(ctx, jobID, "system", sanitizeLog(fmt.Sprintf("%s (exit=%d, duration=%s)", label, result.ExitCode, result.Duration.Round(time.Millisecond)), masks))
	for _, line := range splitLines(sanitizeLog(result.Stdout, masks)) {
		m.appendLog(ctx, jobID, "stdout", line)
	}
	for _, line := range splitLines(sanitizeLog(result.Stderr, masks)) {
		m.appendLog(ctx, jobID, "stderr", line)
	}
}

func (m *SandboxManager) failJob(ctx context.Context, jobID string, state string, message string) error {
	if strings.TrimSpace(state) == "" {
		state = string(models.SandboxJobStateFailed)
	}

	dbCtx, cancel := m.jobWriteContext(ctx)
	defer cancel()

	if _, err := m.pool.Exec(dbCtx, `
		UPDATE sandbox_jobs
		SET state = $2, error = $3, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, jobID, state, message); err != nil {
		return err
	}

	m.appendLog(ctx, jobID, "stderr", message)
	return nil
}

func (m *SandboxManager) claimNextJob(ctx context.Context) (string, error) {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	var jobID string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM sandbox_jobs
		WHERE state = 'queued'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`).Scan(&jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE sandbox_jobs
		SET state = 'running', started_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, jobID); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return jobID, nil
}

func (m *SandboxManager) loadJobExecutionContext(ctx context.Context, jobID string) (*sandboxJobExecutionContext, error) {
	var out sandboxJobExecutionContext
	var prNumber sql.NullInt64
	var prURL sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime

	err := m.pool.QueryRow(ctx, `
		SELECT
			j.id,
			j.organization_id,
			j.repository_id,
			j.requested_by_user_id,
			j.mode,
			j.state,
			j.runtime_type,
			j.sandbox_instance_id,
			j.task_title,
			j.commit_message,
			j.base_branch,
			j.incident_context_json,
			j.incident_prompt,
			j.branch_name,
			j.draft_pr,
			j.pull_request_number,
			j.pull_request_url,
			j.cancel_requested,
			j.error,
			j.created_at,
			j.started_at,
			j.finished_at,
			j.updated_at,
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
		FROM sandbox_jobs j
		JOIN github_repositories r ON r.id = j.repository_id
		JOIN github_installations i ON i.id = r.installation_id
		WHERE j.id = $1
	`, jobID).Scan(
		&out.Job.ID,
		&out.Job.OrganizationID,
		&out.Job.RepositoryID,
		&out.Job.RequestedByUserID,
		&out.Job.Mode,
		&out.Job.State,
		&out.Job.RuntimeType,
		&out.Job.SandboxInstanceID,
		&out.Job.TaskTitle,
		&out.Job.CommitMessage,
		&out.Job.BaseBranch,
		&out.Job.IncidentContext,
		&out.Job.IncidentPrompt,
		&out.Job.BranchName,
		&out.Job.DraftPR,
		&prNumber,
		&prURL,
		&out.Job.CancelRequested,
		&out.Job.Error,
		&out.Job.CreatedAt,
		&startedAt,
		&finishedAt,
		&out.Job.UpdatedAt,
		&out.Repository.ID,
		&out.Repository.OrganizationID,
		&out.Repository.InstallationID,
		&out.Repository.GithubRepositoryID,
		&out.Repository.FullName,
		&out.Repository.DefaultBranch,
		&out.Repository.Private,
		&out.Repository.Archived,
		&out.Repository.Enabled,
		&out.Repository.LastSyncedAt,
		&out.Repository.CreatedAt,
		&out.Repository.UpdatedAt,
		&out.GithubInstallationID,
	)
	if err != nil {
		return nil, err
	}

	if prNumber.Valid {
		v := prNumber.Int64
		out.Job.PullRequestNumber = &v
	}
	if prURL.Valid {
		v := prURL.String
		out.Job.PullRequestURL = &v
	}
	if startedAt.Valid {
		v := startedAt.Time
		out.Job.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		out.Job.FinishedAt = &v
	}

	rows, err := m.pool.Query(ctx, `
		SELECT id, ordinal, command, exit_code, started_at, finished_at
		FROM sandbox_job_commands
		WHERE sandbox_job_id = $1
		ORDER BY ordinal ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commands := make([]models.SandboxJobCommand, 0)
	for rows.Next() {
		var command models.SandboxJobCommand
		var exitCode sql.NullInt32
		var startedAt sql.NullTime
		var finishedAt sql.NullTime
		if err := rows.Scan(&command.ID, &command.Ordinal, &command.Command, &exitCode, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		if exitCode.Valid {
			v := int(exitCode.Int32)
			command.ExitCode = &v
		}
		if startedAt.Valid {
			v := startedAt.Time
			command.StartedAt = &v
		}
		if finishedAt.Valid {
			v := finishedAt.Time
			command.FinishedAt = &v
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.Commands = commands

	return &out, nil
}

func (m *SandboxManager) loadJob(ctx context.Context, organizationID string, jobID string) (*models.SandboxJob, error) {
	var job models.SandboxJob
	var prNumber sql.NullInt64
	var prURL sql.NullString
	var startedAt sql.NullTime
	var finishedAt sql.NullTime

	err := m.pool.QueryRow(ctx, `
		SELECT
			id,
			organization_id,
			repository_id,
			requested_by_user_id,
			mode,
			state,
			runtime_type,
			sandbox_instance_id,
			task_title,
			commit_message,
			base_branch,
			incident_context_json,
			incident_prompt,
			branch_name,
			draft_pr,
			pull_request_number,
			pull_request_url,
			cancel_requested,
			error,
			created_at,
			started_at,
			finished_at,
			updated_at
		FROM sandbox_jobs
		WHERE organization_id = $1
		  AND id = $2
	`, organizationID, jobID).Scan(
		&job.ID,
		&job.OrganizationID,
		&job.RepositoryID,
		&job.RequestedByUserID,
		&job.Mode,
		&job.State,
		&job.RuntimeType,
		&job.SandboxInstanceID,
		&job.TaskTitle,
		&job.CommitMessage,
		&job.BaseBranch,
		&job.IncidentContext,
		&job.IncidentPrompt,
		&job.BranchName,
		&job.DraftPR,
		&prNumber,
		&prURL,
		&job.CancelRequested,
		&job.Error,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if prNumber.Valid {
		v := prNumber.Int64
		job.PullRequestNumber = &v
	}
	if prURL.Valid {
		v := prURL.String
		job.PullRequestURL = &v
	}
	if startedAt.Valid {
		v := startedAt.Time
		job.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		job.FinishedAt = &v
	}

	return &job, nil
}

func (m *SandboxManager) markCommandStarted(ctx context.Context, commandID string) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE sandbox_job_commands
		SET started_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, commandID)
	return err
}

func (m *SandboxManager) markCommandFinished(ctx context.Context, commandID string, exitCode int) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE sandbox_job_commands
		SET exit_code = $2, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, commandID, exitCode)
	return err
}

func (m *SandboxManager) appendLog(ctx context.Context, jobID string, stream string, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	if stream == "" {
		stream = "stdout"
	}

	dbCtx, cancel := m.jobWriteContext(ctx)
	defer cancel()

	_, err := m.pool.Exec(dbCtx, `
		INSERT INTO sandbox_job_logs (sandbox_job_id, seq, stream, message, created_at)
		SELECT $1::VARCHAR(32), COALESCE(MAX(seq), 0) + 1, $2::VARCHAR(16), $3::TEXT, NOW()
		FROM sandbox_job_logs
		WHERE sandbox_job_id = $1::VARCHAR(32)
	`, jobID, stream, message)
	if err != nil {
		m.logger.Warn("failed to append sandbox job log",
			slog.String("job_id", jobID),
			slog.Any("error", err),
		)
	}
}

func (m *SandboxManager) jobWriteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil || ctx.Err() != nil {
		return context.WithTimeout(context.Background(), 5*time.Second)
	}
	return ctx, func() {}
}

func (m *SandboxManager) jobExists(ctx context.Context, organizationID string, jobID string) bool {
	var exists bool
	err := m.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM sandbox_jobs
			WHERE organization_id = $1
			  AND id = $2
		)
	`, organizationID, jobID).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

func (m *SandboxManager) isCancelRequested(ctx context.Context, jobID string) bool {
	var cancelRequested bool
	if err := m.pool.QueryRow(ctx, `
		SELECT cancel_requested
		FROM sandbox_jobs
		WHERE id = $1
	`, jobID).Scan(&cancelRequested); err != nil {
		return false
	}
	return cancelRequested
}

func sanitizeContainerName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "orvo-sandbox"
	}
	replacer := strings.NewReplacer("/", "-", " ", "-", "_", "-", ":", "-", ".", "-")
	raw = replacer.Replace(raw)
	if len(raw) > 63 {
		raw = raw[:63]
	}
	return strings.Trim(raw, "-")
}

func buildBranchName(jobID string) string {
	safeID := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(jobID)), "_", "-")
	safeID = strings.Trim(safeID, "-")
	if safeID == "" {
		safeID = util.GenerateID("job")
	}
	return "orvo/" + safeID
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func sanitizeLog(message string, masks []string) string {
	out := message
	for _, mask := range masks {
		if strings.TrimSpace(mask) == "" {
			continue
		}
		out = strings.ReplaceAll(out, mask, "***")
	}
	return out
}

func splitLines(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitRepositoryFullName(fullName string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(fullName), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository full name: %q", fullName)
	}
	return parts[0], parts[1], nil
}
