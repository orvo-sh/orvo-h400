package workers

import (
	"testing"
	"time"

	"github.com/orvo-sh/orvo/internal/domain/models"
)

func TestCommandTimeoutForJobUsesOpencodeBudgetForAutoResolveCommand(t *testing.T) {
	manager := &SandboxManager{
		config: SandboxManagerConfig{
			CommandTimeout:  90 * time.Second,
			OpencodeTimeout: 120 * time.Second,
		},
	}

	timeout := manager.commandTimeoutForJob(
		models.SandboxJob{Mode: "auto_resolve"},
		models.SandboxJobCommand{Command: models.AutoResolveOpencodeCommandMarker + "; opencode run"},
	)

	if want := 135 * time.Second; timeout != want {
		t.Fatalf("commandTimeoutForJob() = %s, want %s", timeout, want)
	}
}

func TestCommandTimeoutForJobLeavesNormalCommandsAlone(t *testing.T) {
	manager := &SandboxManager{
		config: SandboxManagerConfig{
			CommandTimeout:  90 * time.Second,
			OpencodeTimeout: 120 * time.Second,
		},
	}

	timeout := manager.commandTimeoutForJob(
		models.SandboxJob{Mode: "auto_resolve"},
		models.SandboxJobCommand{Command: "git diff --check"},
	)

	if want := 90 * time.Second; timeout != want {
		t.Fatalf("commandTimeoutForJob() = %s, want %s", timeout, want)
	}
}

func TestCommandTimeoutForJobKeepsLongerGenericTimeout(t *testing.T) {
	manager := &SandboxManager{
		config: SandboxManagerConfig{
			CommandTimeout:  10 * time.Minute,
			OpencodeTimeout: 2 * time.Minute,
		},
	}

	timeout := manager.commandTimeoutForJob(
		models.SandboxJob{Mode: "auto_resolve"},
		models.SandboxJobCommand{Command: models.AutoResolveOpencodeCommandMarker + "; opencode run"},
	)

	if want := 10 * time.Minute; timeout != want {
		t.Fatalf("commandTimeoutForJob() = %s, want %s", timeout, want)
	}
}
