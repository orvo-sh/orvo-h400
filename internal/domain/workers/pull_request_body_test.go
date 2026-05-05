package workers

import (
	"strings"
	"testing"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func TestParseAutoResolveIncidentContext(t *testing.T) {
	raw := `{"log":{"id":"log_123","service_name":"payments","body":"database timeout","timestamp":"2026-03-27T10:00:00Z"},"trace_spans":[{"id":"spn_1"}],"nearby_service_errors":[{"id":"log_456"}],"repository_full_name":"acme/payments","related_repositories":[{"service_name":"ios","repository_id":"repo_2","repository_full_name":"acme/ios","reason":"observed in trace with payments"}]}`
	out := parseAutoResolveIncidentContext(raw)
	if out == nil {
		t.Fatalf("expected incident context to parse")
	}
	if out.Log.ID != "log_123" {
		t.Fatalf("unexpected log id: %q", out.Log.ID)
	}
	if out.RepositoryFullName != "acme/payments" {
		t.Fatalf("unexpected repository full name: %q", out.RepositoryFullName)
	}
	if len(out.RelatedRepositories) != 1 {
		t.Fatalf("expected related repositories to parse")
	}
}

func TestCollectValidationLines(t *testing.T) {
	exitCode := 0
	lines := collectValidationLines([]models.SandboxJobCommand{
		{Ordinal: 2, Command: "go test ./...", ExitCode: &exitCode},
	}, false)
	if len(lines) != 1 {
		t.Fatalf("expected one validation line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "go test ./...") {
		t.Fatalf("expected command text in validation line: %q", lines[0])
	}
}

func TestCollectValidationLinesAutoResolveSkipsSetupCommands(t *testing.T) {
	exitCode := 0
	lines := collectValidationLines([]models.SandboxJobCommand{
		{Ordinal: 1, Command: "prepare opencode", ExitCode: &exitCode},
		{Ordinal: 3, Command: "run opencode", ExitCode: &exitCode},
		{Ordinal: 4, Command: "git diff --check && git status --short", ExitCode: &exitCode},
	}, true)
	if len(lines) != 1 {
		t.Fatalf("expected one validation line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "git diff --check") {
		t.Fatalf("unexpected validation line: %q", lines[0])
	}
}

func TestExtractOpencodeTextEvent(t *testing.T) {
	message := `{"type":"text","part":{"text":"\n\nFixed. Removed the bad validation and kept the path-based delete flow.\n"}}`
	out := extractOpencodeTextEvent(message)
	if !strings.Contains(out, "Fixed.") {
		t.Fatalf("expected extracted text, got %q", out)
	}
}
