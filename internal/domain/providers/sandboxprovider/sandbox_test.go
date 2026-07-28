package sandboxprovider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateContainerSessionCopiesOpencodeConfigIntoWritableDirectory(t *testing.T) {
	tempDir := t.TempDir()
	argsFile := filepath.Join(tempDir, "args")
	dockerStub := filepath.Join(tempDir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\nprintf 'container-id\\n'\n"
	if err := os.WriteFile(dockerStub, []byte(script), 0o700); err != nil {
		t.Fatalf("write docker stub: %v", err)
	}

	p := &provider{cfg: Config{
		DockerBinary:      dockerStub,
		OpencodeConfigDir: "/host/opencode",
	}}
	if _, err := p.createContainerSession(context.Background(), "test-session", "test-image", "/workspace", "", ""); err != nil {
		t.Fatalf("createContainerSession() error = %v", err)
	}

	rawArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read docker arguments: %v", err)
	}
	args := string(rawArgs)
	if !strings.Contains(args, "/host/opencode:/opt/orvo/opencode-config:ro") {
		t.Fatalf("docker arguments do not contain read-only source mount: %s", args)
	}
	if !strings.Contains(args, "cp -R /opt/orvo/opencode-config/. /root/.config/opencode/") {
		t.Fatalf("docker startup does not copy config to writable directory: %s", args)
	}
	if strings.Contains(args, "/host/opencode:/root/.config/opencode:ro") {
		t.Fatalf("docker arguments still mount the active config directory read-only: %s", args)
	}
}
