package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/orvo-sh/orvo/internal/config"
	"github.com/orvo-sh/orvo/internal/domain/providers/githubprovider"
	"github.com/orvo-sh/orvo/internal/domain/providers/sandboxprovider"
	"github.com/orvo-sh/orvo/internal/domain/services/githubservice"
	"github.com/orvo-sh/orvo/internal/domain/services/remediationservice"
	"github.com/orvo-sh/orvo/internal/domain/services/traceservice"
	"github.com/orvo-sh/orvo/internal/domain/workers"
	"github.com/orvo-sh/orvo/internal/infra/postgres"
)

const targetLogID = "log_01kjcvyjr8mq2x3pfrzm2bffyq"

func main() {
	_ = godotenv.Load(".env")
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	pg, err := postgres.New(ctx, postgres.Config{URL: cfg.Postgres.URL})
	if err != nil {
		panic(err)
	}
	defer pg.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var orgID string
	if err := pg.Pool().QueryRow(ctx, `
		SELECT organization_id
		FROM (
			SELECT organization_id, id FROM logs_hot
			UNION ALL
			SELECT organization_id, id FROM logs_restored
		) AS logs
		WHERE id = $1
		LIMIT 1
	`, targetLogID).Scan(&orgID); err != nil {
		panic(fmt.Errorf("load target log org: %w", err))
	}

	var userID string
	if err := pg.Pool().QueryRow(ctx, `
		SELECT user_id
		FROM organization_members
		WHERE organization_id = $1
		  AND role IN ('owner', 'admin')
		ORDER BY created_at ASC
		LIMIT 1
	`, orgID).Scan(&userID); err != nil {
		panic(fmt.Errorf("load org owner/admin: %w", err))
	}

	privateKey := strings.TrimSpace(cfg.GitHub.AppPrivateKey)
	if privateKey == "" && strings.TrimSpace(cfg.GitHub.AppPrivateKeyFile) != "" {
		keyBytes, readErr := os.ReadFile(strings.TrimSpace(cfg.GitHub.AppPrivateKeyFile))
		if readErr != nil {
			panic(fmt.Errorf("read github private key: %w", readErr))
		}
		privateKey = string(keyBytes)
	}

	githubProvider, err := githubprovider.New(githubprovider.Config{
		AppID:         cfg.GitHub.AppID,
		AppSlug:       cfg.GitHub.AppSlug,
		PrivateKeyPEM: privateKey,
		WebhookSecret: cfg.GitHub.WebhookSecret,
		APIBaseURL:    cfg.GitHub.APIBaseURL,
		AppBaseURL:    cfg.GitHub.AppBaseURL,
	})
	if err != nil {
		panic(fmt.Errorf("github provider: %w", err))
	}

	githubSvc := githubservice.New(pg, logger, githubProvider, githubservice.Config{
		SetupRedirectURL: cfg.GitHub.SetupRedirectURL,
		StateSecret:      cfg.GitHub.StateSecret,
		StateTTL:         10 * time.Minute,
	})
	traceSvc := traceservice.New(pg, logger)
	sandboxProv := sandboxprovider.New(sandboxprovider.Config{
		DockerBinary:        cfg.Sandbox.DockerBinary,
		DefaultImage:        cfg.Sandbox.DefaultImage,
		WorkingDir:          cfg.Sandbox.WorkingDir,
		CPULimit:            cfg.Sandbox.CPULimit,
		MemoryLimit:         cfg.Sandbox.MemoryLimit,
		FallbackToContainer: cfg.Sandbox.FallbackToContainer,
	})
	sandboxMgr := workers.NewSandboxManager(logger, pg.Pool(), githubProvider, githubSvc, sandboxProv, workers.SandboxManagerConfig{
		DefaultImage:     cfg.Sandbox.DefaultImage,
		WorkingDir:       cfg.Sandbox.WorkingDir,
		CPULimit:         cfg.Sandbox.CPULimit,
		MemoryLimit:      cfg.Sandbox.MemoryLimit,
		JobTimeout:       cfg.Sandbox.JobTimeout,
		CommandTimeout:   cfg.Sandbox.CommandTimeout,
		BootstrapTimeout: cfg.Sandbox.BootstrapTimeout,
		GitAuthorName:    cfg.Sandbox.GitAuthorName,
		GitAuthorEmail:   cfg.Sandbox.GitAuthorEmail,
	})

	remediationSvc := remediationservice.New(
		pg,
		logger,
		githubSvc,
		traceSvc,
		sandboxMgr,
		remediationservice.Config{
			OpencodeCommand: cfg.Sandbox.OpencodeCommand,
			OpencodeModel:   cfg.Sandbox.OpencodeModel,
			OpencodeAgent:   cfg.Sandbox.OpencodeAgent,
			OpencodeTimeout: cfg.Sandbox.OpencodeTimeout,
		},
	)

	job, appErr := remediationSvc.RunAutoResolve(ctx, remediationservice.RunAutoResolveInput{
		OrganizationID: orgID,
		LogID:          targetLogID,
		UserID:         userID,
	})
	if appErr != nil {
		panic(fmt.Errorf("run auto resolve: %s", appErr.Error()))
	}
	fmt.Printf("created job=%s org=%s user=%s image=%s\n", job.ID, orgID, userID, cfg.Sandbox.DefaultImage)

	pollCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			_ = sandboxMgr.ProcessQueued(pollCtx)
			select {
			case <-pollCtx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()

	cursor := int64(0)
	start := time.Now()
	for {
		current, appErr := sandboxMgr.GetJob(ctx, orgID, job.ID)
		if appErr != nil {
			fmt.Printf("get job error: %s\n", appErr.Code())
			break
		}
		logsOut, logsErr := sandboxMgr.GetLogs(ctx, orgID, job.ID, cursor, 500)
		if logsErr != nil {
			fmt.Printf("get logs error: %s\n", logsErr.Code())
			break
		}
		if len(logsOut.Logs) > 0 {
			for _, line := range logsOut.Logs {
				fmt.Printf("[%s] %s\n", line.Stream, line.Message)
				cursor = line.Seq
			}
		}

		fmt.Printf("state=%s elapsed=%s logs=%d\n", current.State, time.Since(start).Round(time.Second), cursor)
		switch current.State {
		case "succeeded", "failed", "cancelled", "timed_out":
			fmt.Printf("final error=%q pr=%v\n", current.Error, current.PullRequestURL)
			return
		}

		if time.Since(start) > 10*time.Minute {
			fmt.Println("probe timeout reached")
			return
		}

		time.Sleep(2 * time.Second)
	}
}
