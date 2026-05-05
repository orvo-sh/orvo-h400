package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
	"github.com/orvo-sh/orvo/internal/domain/providers/sandboxprovider"
)

type autoResolveIncidentContext struct {
	GeneratedAt            string                                `json:"generated_at"`
	Log                    models.LogRecord                      `json:"log"`
	TraceSpans             []models.Span                         `json:"trace_spans"`
	NearbyServiceErrors    []models.LogRecord                    `json:"nearby_service_errors"`
	RepositoryFullName     string                                `json:"repository_full_name"`
	RelatedRepositories    []models.AutoResolveRepositoryContext `json:"related_repositories"`
	SuggestedBaseBranch    string                                `json:"suggested_base_branch"`
	SuggestedTaskTitle     string                                `json:"suggested_task_title"`
	SuggestedCommitMessage string                                `json:"suggested_commit_message"`
}

func (m *SandboxManager) buildPullRequestBody(
	ctx context.Context,
	session *sandboxprovider.Session,
	jobID string,
	jobCtx *sandboxJobExecutionContext,
	autoResolveSummary string,
) string {
	diffStat := strings.TrimSpace(m.collectGitOutput(ctx, session, "cd repo && git show --stat --format= HEAD", 20*time.Second))
	changedFiles := collectLines(m.collectGitOutput(ctx, session, "cd repo && git diff-tree --no-commit-id --name-only -r HEAD", 20*time.Second))
	incident := parseAutoResolveIncidentContext(jobCtx.Job.IncidentContext)

	lines := []string{
		"## Summary",
		fmt.Sprintf("- AI-generated remediation draft produced by Orvo sandbox job `%s`.", jobID),
		fmt.Sprintf("- Pull request target: `%s` -> `%s`.", fallbackString(jobCtx.Job.BranchName, "unknown"), fallbackString(jobCtx.Job.BaseBranch, "unknown")),
		fmt.Sprintf("- Commit message: `%s`.", jobCtx.Job.CommitMessage),
	}

	if incident != nil {
		lines = append(lines,
			fmt.Sprintf("- Service: `%s`.", fallbackString(incident.Log.ServiceName, "unknown")),
			fmt.Sprintf("- Incident log: `%s` at `%s`.", incident.Log.ID, incident.Log.Timestamp.UTC().Format(time.RFC3339)),
			fmt.Sprintf("- Error excerpt: `%s`.", truncatePRLine(compactWhitespace(incident.Log.Body), 180)),
		)
		if strings.TrimSpace(incident.Log.TraceID) != "" {
			lines = append(lines, fmt.Sprintf("- Trace: `%s`.", incident.Log.TraceID))
		}
	}

	autoResolveSummary = strings.TrimSpace(autoResolveSummary)
	if autoResolveSummary != "" {
		lines = append(lines, "", "## Error and Fix", autoResolveSummary)
	}

	lines = append(lines, "", "## Changes")
	if len(changedFiles) == 0 {
		lines = append(lines, "- Git reported changes, but the changed file list could not be collected.")
	} else {
		for _, file := range changedFiles {
			lines = append(lines, fmt.Sprintf("- `%s`", file))
		}
	}
	if diffStat != "" {
		lines = append(lines, "", "```text", diffStat, "```")
	}

	lines = append(lines, "", "## Validation")
	validationLines := collectValidationLines(jobCtx.Commands, strings.EqualFold(jobCtx.Job.Mode, "auto_resolve"))
	if len(validationLines) == 0 {
		lines = append(lines, "- No validation commands were recorded.")
	} else {
		lines = append(lines, validationLines...)
	}

	if incident != nil {
		lines = append(lines,
			"",
			"## Incident Context",
			fmt.Sprintf("- Trace spans reviewed: `%d`.", len(incident.TraceSpans)),
			fmt.Sprintf("- Nearby error logs reviewed: `%d`.", len(incident.NearbyServiceErrors)),
			fmt.Sprintf("- Repository mapping: `%s`.", fallbackString(incident.RepositoryFullName, jobCtx.Repository.FullName)),
		)
		if len(incident.RelatedRepositories) > 0 {
			lines = append(lines, "- Related repositories reviewed for context:")
			for _, repo := range incident.RelatedRepositories {
				lines = append(lines, fmt.Sprintf("- Related repo: `%s` for service `%s` (%s).", repo.RepositoryFullName, repo.ServiceName, repo.Reason))
			}
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (m *SandboxManager) collectGitOutput(
	ctx context.Context,
	session *sandboxprovider.Session,
	command string,
	timeout time.Duration,
) string {
	result, err := m.sandboxProvider.Exec(ctx, session, command, timeout)
	if err != nil || result == nil || result.ExitCode != 0 {
		return ""
	}
	return result.Stdout
}

func parseAutoResolveIncidentContext(raw string) *autoResolveIncidentContext {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var out autoResolveIncidentContext
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return &out
}

func collectLines(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func collectValidationLines(commands []models.SandboxJobCommand, autoResolve bool) []string {
	lines := make([]string, 0)
	for _, command := range commands {
		if command.ExitCode == nil {
			continue
		}
		if autoResolve && command.Ordinal <= 3 {
			continue
		}
		lines = append(lines, fmt.Sprintf("- `command #%d` exited with `%d`: `%s`", command.Ordinal, *command.ExitCode, truncatePRLine(compactWhitespace(command.Command), 120)))
	}
	return lines
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncatePRLine(value string, max int) string {
	if max <= 3 || len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}

func fallbackString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
