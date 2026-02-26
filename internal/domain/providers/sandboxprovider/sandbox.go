package sandboxprovider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type RuntimeType string

const (
	RuntimeDockerSandbox   RuntimeType = "docker_sandbox"
	RuntimeDockerContainer RuntimeType = "docker_container"
)

type Config struct {
	DockerBinary        string
	DefaultImage        string
	WorkingDir          string
	CPULimit            string
	MemoryLimit         string
	FallbackToContainer bool
}

type CreateSessionInput struct {
	Name        string
	Image       string
	WorkingDir  string
	CPULimit    string
	MemoryLimit string
}

type Session struct {
	ID         string
	Runtime    RuntimeType
	WorkingDir string
}

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
	Duration time.Duration
}

type Provider interface {
	Enabled() bool
	Runtime() RuntimeType
	PrePull(ctx context.Context, image string) error
	CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error)
	Exec(ctx context.Context, session *Session, command string, timeout time.Duration) (*ExecResult, error)
	Destroy(ctx context.Context, session *Session) error
}

type provider struct {
	cfg              Config
	preferredRuntime RuntimeType
}

func New(cfg Config) Provider {
	if strings.TrimSpace(cfg.DockerBinary) == "" {
		cfg.DockerBinary = "docker"
	}
	if strings.TrimSpace(cfg.DefaultImage) == "" {
		cfg.DefaultImage = "mirror.gcr.io/library/node:25"
	}
	if strings.TrimSpace(cfg.WorkingDir) == "" {
		cfg.WorkingDir = "/workspace"
	}

	runtime := RuntimeDockerContainer
	if hasDockerSandbox(cfg.DockerBinary) {
		runtime = RuntimeDockerSandbox
	}

	return &provider{
		cfg:              cfg,
		preferredRuntime: runtime,
	}
}

func (p *provider) Enabled() bool {
	_, err := exec.LookPath(p.cfg.DockerBinary)
	return err == nil
}

func (p *provider) Runtime() RuntimeType {
	return p.preferredRuntime
}

func (p *provider) PrePull(ctx context.Context, image string) error {
	if !p.Enabled() {
		return fmt.Errorf("docker CLI is not available")
	}

	targetImage := strings.TrimSpace(image)
	if targetImage == "" {
		targetImage = p.cfg.DefaultImage
	}
	if targetImage == "" {
		return fmt.Errorf("image is required")
	}

	if p.imageExistsLocally(ctx, targetImage) {
		return nil
	}

	cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, "pull", targetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker pull failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func (p *provider) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	if !p.Enabled() {
		return nil, fmt.Errorf("docker CLI is not available")
	}

	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("sandbox session name is required")
	}

	image := strings.TrimSpace(input.Image)
	if image == "" {
		image = p.cfg.DefaultImage
	}

	workingDir := strings.TrimSpace(input.WorkingDir)
	if workingDir == "" {
		workingDir = p.cfg.WorkingDir
	}

	cpu := strings.TrimSpace(input.CPULimit)
	if cpu == "" {
		cpu = p.cfg.CPULimit
	}
	memory := strings.TrimSpace(input.MemoryLimit)
	if memory == "" {
		memory = p.cfg.MemoryLimit
	}

	if p.preferredRuntime == RuntimeDockerSandbox {
		session, err := p.createSandboxSession(ctx, input.Name, image, workingDir, cpu, memory)
		if err == nil {
			return session, nil
		}
		if !p.cfg.FallbackToContainer {
			return nil, err
		}
	}

	return p.createContainerSession(ctx, input.Name, image, workingDir, cpu, memory)
}

func (p *provider) Exec(ctx context.Context, session *Session, command string, timeout time.Duration) (*ExecResult, error) {
	if session == nil {
		return nil, fmt.Errorf("sandbox session is nil")
	}

	execCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var args []string
	switch session.Runtime {
	case RuntimeDockerSandbox:
		args = []string{"sandbox", "exec", session.ID, "sh", "-lc", command}
	default:
		args = []string{"exec", session.ID, "sh", "-lc", command}
	}

	start := time.Now()
	cmd := exec.CommandContext(execCtx, p.cfg.DockerBinary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)
	result := &ExecResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err == nil {
		return result, nil
	}

	if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = 124
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return nil, err
}

func (p *provider) Destroy(ctx context.Context, session *Session) error {
	if session == nil || strings.TrimSpace(session.ID) == "" {
		return nil
	}

	switch session.Runtime {
	case RuntimeDockerSandbox:
		cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, "sandbox", "rm", "-f", session.ID)
		if err := cmd.Run(); err != nil && !isNotFoundError(err) {
			return err
		}
	default:
		cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, "rm", "-f", session.ID)
		if err := cmd.Run(); err != nil && !isNotFoundError(err) {
			return err
		}
	}

	return nil
}

func (p *provider) createSandboxSession(ctx context.Context, name string, image string, workingDir string, cpu string, memory string) (*Session, error) {
	args := []string{"sandbox", "create", "--name", name, "--image", image}
	if strings.TrimSpace(workingDir) != "" {
		args = append(args, "--workdir", workingDir)
	}
	if strings.TrimSpace(cpu) != "" {
		args = append(args, "--cpus", cpu)
	}
	if strings.TrimSpace(memory) != "" {
		args = append(args, "--memory", memory)
	}

	cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("create docker sandbox session: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	sessionID := strings.TrimSpace(string(output))
	if sessionID == "" {
		sessionID = name
	}

	return &Session{
		ID:         sessionID,
		Runtime:    RuntimeDockerSandbox,
		WorkingDir: workingDir,
	}, nil
}

func (p *provider) createContainerSession(ctx context.Context, name string, image string, workingDir string, cpu string, memory string) (*Session, error) {
	args := []string{"run", "-d", "--rm", "--name", name, "-w", workingDir}
	if strings.TrimSpace(cpu) != "" {
		args = append(args, "--cpus", cpu)
	}
	if strings.TrimSpace(memory) != "" {
		args = append(args, "--memory", memory)
	}
	args = append(args, image, "sh", "-lc", "sleep infinity")

	cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("create docker container session: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	containerID := strings.TrimSpace(string(output))
	if containerID == "" {
		containerID = name
	}

	return &Session{
		ID:         containerID,
		Runtime:    RuntimeDockerContainer,
		WorkingDir: workingDir,
	}, nil
}

func hasDockerSandbox(binary string) bool {
	cmd := exec.Command(binary, "sandbox", "--help")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (p *provider) imageExistsLocally(ctx context.Context, image string) bool {
	cmd := exec.CommandContext(ctx, p.cfg.DockerBinary, "image", "inspect", image)
	return cmd.Run() == nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such container") || strings.Contains(msg, "not found")
}
