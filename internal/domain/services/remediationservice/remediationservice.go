package remediationservice

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/orvo-sh/orvo/internal/domain/errs"
	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/services/githubservice"
	"github.com/orvo-sh/orvo/internal/domain/services/metricservice"
	"github.com/orvo-sh/orvo/internal/domain/services/traceservice"
	"github.com/orvo-sh/orvo/internal/domain/workers"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
	"github.com/orvo-sh/orvo/pkg/apperr"
)

const (
	defaultNearbyErrorLimit = 20
	retiredMinimaxFreeModel = "opencode/minimax-m2.5-free"
	replacementFreeModel    = "opencode/mimo-v2.5-free"
)

type Service interface {
	ListMappings(ctx context.Context, organizationID string) ([]models.ServiceRemediationMapping, apperr.Error)
	UpsertMapping(ctx context.Context, input UpsertMappingInput) (*models.ServiceRemediationMapping, apperr.Error)
	DeleteMapping(ctx context.Context, organizationID string, serviceName string) apperr.Error
	ListAutoResolveThresholds(ctx context.Context, organizationID string) ([]models.AutoResolveThreshold, apperr.Error)
	UpsertAutoResolveThreshold(ctx context.Context, input UpsertAutoResolveThresholdInput) (*models.AutoResolveThreshold, apperr.Error)
	DeleteAutoResolveThreshold(ctx context.Context, organizationID string, serviceName string) apperr.Error
	PreviewAutoResolve(ctx context.Context, input PreviewAutoResolveInput) (*models.AutoResolvePreview, apperr.Error)
	RunAutoResolve(ctx context.Context, input RunAutoResolveInput) (*models.SandboxJob, apperr.Error)
	ProcessAutoResolveThresholds(ctx context.Context) error
}

type Config struct {
	OpencodeCommand    string
	OpencodeModel      string
	OpencodeVariant    string
	OpencodeAgent      string
	ContextWindow      time.Duration
	NearbyErrorLimit   int
	MaxContextBytes    int
	ValidationCommands []string
}

type UpsertMappingInput struct {
	OrganizationID string
	ServiceName    string
	RepositoryID   string
	UserID         string
}

type PreviewAutoResolveInput struct {
	OrganizationID string
	LogID          string
}

type RunAutoResolveInput struct {
	OrganizationID string
	LogID          string
	UserID         string
}

type sandboxJobCreator interface {
	CreateJob(ctx context.Context, input workers.CreateSandboxJobInput) (*models.SandboxJob, apperr.Error)
}

type service struct {
	pg                  *postgres.DB
	logger              *slog.Logger
	githubService       githubservice.Service
	metricService       metricQuerier
	traceService        traceservice.Service
	sandboxJobs         sandboxJobCreator
	config              Config
	thresholdSchemaOnce sync.Once
	thresholdSchemaErr  error
}

func New(
	pg *postgres.DB,
	logger *slog.Logger,
	githubService githubservice.Service,
	metricService metricservice.Service,
	traceService traceservice.Service,
	sandboxJobs sandboxJobCreator,
	config Config,
) Service {
	if strings.TrimSpace(config.OpencodeCommand) == "" {
		config.OpencodeCommand = "opencode"
	}
	if config.ContextWindow <= 0 {
		config.ContextWindow = 15 * time.Minute
	}
	if config.NearbyErrorLimit <= 0 {
		config.NearbyErrorLimit = defaultNearbyErrorLimit
	}
	if config.MaxContextBytes <= 0 {
		config.MaxContextBytes = 256 * 1024
	}
	if len(config.ValidationCommands) == 0 {
		config.ValidationCommands = []string{
			"if [ -f go.mod ] && command -v go >/dev/null 2>&1; then go test ./...; " +
				"elif [ -f pnpm-lock.yaml ] && command -v pnpm >/dev/null 2>&1; then pnpm test --if-present; " +
				"elif [ -f package.json ] && command -v npm >/dev/null 2>&1; then npm test --if-present; " +
				"else echo 'no validation profile detected'; fi",
		}
	}

	return &service{
		pg:            pg,
		logger:        logger.With("module", "remediation_service"),
		githubService: githubService,
		metricService: metricService,
		traceService:  traceService,
		sandboxJobs:   sandboxJobs,
		config:        config,
	}
}

type autoResolvePlan struct {
	Preview     models.AutoResolvePreview
	ContextJSON string
	Prompt      string
}

func (s *service) PreviewAutoResolve(ctx context.Context, input PreviewAutoResolveInput) (*models.AutoResolvePreview, apperr.Error) {
	plan, appErr := s.buildAutoResolvePlan(ctx, input.OrganizationID, input.LogID)
	if appErr != nil {
		return nil, appErr
	}
	return &plan.Preview, nil
}

func (s *service) RunAutoResolve(ctx context.Context, input RunAutoResolveInput) (*models.SandboxJob, apperr.Error) {
	if strings.TrimSpace(s.config.OpencodeCommand) == "" || strings.TrimSpace(s.config.OpencodeModel) == "" {
		return nil, errs.ErrAutoResolveOpencodeMissing
	}

	plan, appErr := s.buildAutoResolvePlan(ctx, input.OrganizationID, input.LogID)
	if appErr != nil {
		return nil, appErr
	}

	opencodeArgs := []string{
		shellQuote(s.config.OpencodeCommand),
		"run",
		"--format",
		"json",
		"--model",
		shellQuote(supportedOpencodeModel(s.config.OpencodeModel)),
	}
	if strings.TrimSpace(s.config.OpencodeVariant) != "" {
		opencodeArgs = append(opencodeArgs, "--variant", shellQuote(s.config.OpencodeVariant))
	}
	if strings.TrimSpace(s.config.OpencodeAgent) != "" {
		opencodeArgs = append(opencodeArgs, "--agent", shellQuote(s.config.OpencodeAgent))
	}
	opencodeArgs = append(opencodeArgs, shellQuote(plan.Prompt))
	opencodeRunCommand := strings.Join(opencodeArgs, " ")
	ensureOpencodeInstalledCommand := buildEnsureOpencodeInstalledCommand(s.config.OpencodeCommand)
	blockPackageManagersCommand := buildBlockPackageManagersCommand()

	commands := []string{
		ensureOpencodeInstalledCommand,
		blockPackageManagersCommand,
		fmt.Sprintf(
			"%s; if ! command -v %s >/dev/null 2>&1; then echo '%s not found in sandbox after install step'; exit 127; fi && PATH=\"$PWD/.orvo/opencode-bin:$PATH\" %s",
			models.AutoResolveOpencodeCommandMarker,
			shellQuote(s.config.OpencodeCommand),
			s.config.OpencodeCommand,
			opencodeRunCommand,
		),
	}
	commands = append(commands, s.config.ValidationCommands...)

	job, appErr := s.sandboxJobs.CreateJob(ctx, workers.CreateSandboxJobInput{
		OrganizationID:      input.OrganizationID,
		RepositoryID:        plan.Preview.RepositoryID,
		RequestedBy:         input.UserID,
		BaseBranch:          plan.Preview.BaseBranch,
		TaskTitle:           plan.Preview.TaskTitle,
		CommitMessage:       plan.Preview.CommitMessage,
		Commands:            commands,
		DraftPR:             true,
		Mode:                "auto_resolve",
		IncidentContextJSON: plan.ContextJSON,
		IncidentPrompt:      plan.Prompt,
	})
	if appErr != nil {
		return nil, appErr
	}
	return job, nil
}

func supportedOpencodeModel(model string) string {
	model = strings.TrimSpace(model)
	if strings.EqualFold(model, retiredMinimaxFreeModel) {
		return replacementFreeModel
	}
	return model
}

func (s *service) buildAutoResolvePlan(ctx context.Context, organizationID string, logID string) (*autoResolvePlan, apperr.Error) {
	logRecord, err := s.loadLogByID(ctx, organizationID, logID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrNotFound
		}
		s.logger.ErrorContext(ctx, "failed to load log by id", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	if !isErrorLog(logRecord) {
		return nil, errs.ErrAutoResolveNotError
	}

	mapping, appErr := s.loadMappingForService(ctx, organizationID, logRecord.ServiceName)
	if appErr != nil {
		return nil, appErr
	}

	traceSpans := make([]models.Span, 0)
	if strings.TrimSpace(logRecord.TraceID) != "" && s.traceService != nil {
		traceOut, traceErr := s.traceService.GetTrace(ctx, organizationID, logRecord.TraceID)
		if traceErr == nil && traceOut != nil {
			traceSpans = traceOut.Spans
		}
	}

	nearbyErrors, err := s.loadNearbyServiceErrors(ctx, organizationID, logRecord.ServiceName, logRecord.Timestamp, logRecord.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to load nearby error logs", slog.Any("error", err))
		return nil, errs.ErrInternal
	}

	shortID := shortLogID(logRecord.ID)
	baseBranch := "main"
	if repo, repoErr := s.githubService.GetAutomationRepository(ctx, organizationID, mapping.RepositoryID); repoErr == nil && repo != nil {
		baseBranch = repo.Repository.DefaultBranch
		if strings.TrimSpace(baseBranch) == "" {
			baseBranch = "main"
		}
	}

	preview := models.AutoResolvePreview{
		LogID:              logRecord.ID,
		ServiceName:        logRecord.ServiceName,
		RepositoryID:       mapping.RepositoryID,
		RepositoryFullName: mapping.RepositoryFullName,
		BaseBranch:         baseBranch,
		TaskTitle:          fmt.Sprintf("Auto-resolve %s: %s", logRecord.ServiceName, summarizeLogBody(logRecord.Body)),
		CommitMessage:      fmt.Sprintf("fix(%s): auto-resolve error %s", normalizeForCommitScope(logRecord.ServiceName), shortID),
		ValidationCommands: append([]string{}, s.config.ValidationCommands...),
		ContextSummary: models.AutoResolveContextSummary{
			TraceSpanCount:      len(traceSpans),
			NearbyErrorLogCount: len(nearbyErrors),
		},
	}
	preview.RelatedRepositories = s.loadRelatedRepositoryContexts(
		ctx,
		organizationID,
		logRecord.ServiceName,
		traceSpans,
		mapping.RepositoryID,
	)

	contextPayload := map[string]any{
		"generated_at":             time.Now().UTC().Format(time.RFC3339),
		"log":                      logRecord,
		"trace_spans":              traceSpans,
		"nearby_service_errors":    nearbyErrors,
		"repository_full_name":     mapping.RepositoryFullName,
		"related_repositories":     preview.RelatedRepositories,
		"suggested_base_branch":    baseBranch,
		"suggested_task_title":     preview.TaskTitle,
		"suggested_commit_message": preview.CommitMessage,
	}
	contextJSON, marshalErr := json.MarshalIndent(contextPayload, "", "  ")
	if marshalErr != nil {
		s.logger.ErrorContext(ctx, "failed to marshal auto resolve context", slog.Any("error", marshalErr))
		return nil, errs.ErrInternal
	}
	if len(contextJSON) > s.config.MaxContextBytes {
		return nil, errs.ErrAutoResolveContextTooLarge
	}

	prompt := strings.TrimSpace(fmt.Sprintf(
		"Use .orvo/incident-context.json to diagnose and fix the error. "+
			"Make the smallest safe code changes to resolve the issue in /workspace/repo only. "+
			"If .orvo/related-repositories.json exists, inspect those repositories at /workspace/related for read-only context only; do not edit files outside /workspace/repo and do not attempt a multi-repo patch. "+
			"Before stopping, write a short markdown summary to .orvo/auto-resolve-summary.md with sections: Error, Root Cause, Fix, Validation, Cross-Service Notes. "+
			"Do not install dependencies or run package managers (npm/pnpm/yarn/bun/apt/apk/dnf/yum/pip). "+
			"Do not run long test suites; prefer quick sanity checks only. "+
			"Stop after applying the minimal patch. "+
			"Service: %s. Log ID: %s.",
		logRecord.ServiceName,
		logRecord.ID,
	))

	return &autoResolvePlan{
		Preview:     preview,
		ContextJSON: string(contextJSON),
		Prompt:      prompt,
	}, nil
}

func shortLogID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func summarizeLogBody(body string) string {
	body = strings.Join(strings.Fields(strings.TrimSpace(body)), " ")
	if body == "" {
		return "incident remediation"
	}
	if len(body) <= 72 {
		return body
	}
	return body[:69] + "..."
}

func normalizeForCommitScope(service string) string {
	service = strings.TrimSpace(strings.ToLower(service))
	if service == "" {
		return "service"
	}
	service = strings.ReplaceAll(service, " ", "-")
	return service
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func buildEnsureOpencodeInstalledCommand(opencodeCommand string) string {
	quoted := shellQuote(strings.TrimSpace(opencodeCommand))
	installCommand := "" +
		"if command -v npm >/dev/null 2>&1; then npm install -g opencode-ai --no-audit --no-fund; " +
		"elif command -v apt-get >/dev/null 2>&1; then apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends npm ca-certificates && npm install -g opencode-ai --no-audit --no-fund; " +
		"elif command -v apk >/dev/null 2>&1; then apk add --no-cache nodejs npm ca-certificates && npm install -g opencode-ai --no-audit --no-fund; " +
		"elif command -v dnf >/dev/null 2>&1; then dnf install -y nodejs npm ca-certificates && npm install -g opencode-ai --no-audit --no-fund; " +
		"elif command -v yum >/dev/null 2>&1; then yum install -y nodejs npm ca-certificates && npm install -g opencode-ai --no-audit --no-fund; " +
		"else echo \"unable to install opencode (no supported package manager)\"; exit 127; fi"

	return fmt.Sprintf("if command -v %s >/dev/null 2>&1; then echo 'opencode available'; else ", quoted) +
		"if command -v timeout >/dev/null 2>&1; then timeout 180s sh -lc " + shellQuote(installCommand) + "; " +
		"else sh -lc " + shellQuote(installCommand) + "; fi; fi"
}

func buildBlockPackageManagersCommand() string {
	return "mkdir -p .orvo/opencode-bin && " +
		"for cmd in npm pnpm yarn bun apt-get apk dnf yum pip pip3; do " +
		"printf '#!/bin/sh\\necho \"package manager disabled during opencode run: %s\" >&2\\nexit 127\\n' \"$cmd\" > .orvo/opencode-bin/$cmd && chmod +x .orvo/opencode-bin/$cmd; " +
		"done"
}

func serviceNameFromLog(record *models.LogRecord) string {
	serviceName := strings.TrimSpace(record.ServiceName)
	if serviceName != "" {
		return serviceName
	}
	if record.ResourceAttributes != nil {
		if v := strings.TrimSpace(record.ResourceAttributes["service.name"]); v != "" {
			return v
		}
	}
	return ""
}
