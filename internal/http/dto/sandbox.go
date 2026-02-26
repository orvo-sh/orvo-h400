package dto

import "github.com/orvo-sh/orvo/internal/domain/models"

type CreateSandboxJobInput struct {
	OrganizationID string `path:"organization_id"`
	Body           struct {
		RepositoryID  string   `json:"repository_id" minLength:"1"`
		BaseBranch    string   `json:"base_branch,omitempty"`
		TaskTitle     string   `json:"task_title,omitempty"`
		CommitMessage string   `json:"commit_message" minLength:"1"`
		Commands      []string `json:"commands" minItems:"1"`
		DraftPR       *bool    `json:"draft_pr,omitempty"`
	}
}

type CreateSandboxJobOutput struct {
	Body struct {
		JobID string `json:"job_id"`
	}
}

type GetSandboxJobInput struct {
	OrganizationID string `path:"organization_id"`
	JobID          string `path:"job_id"`
}

type ListSandboxJobsInput struct {
	OrganizationID string `path:"organization_id"`
	State          string `query:"state" doc:"Comma-separated job states to include (e.g. queued,running)" required:"false"`
	Limit          int    `query:"limit" doc:"Max jobs to return (default 20, max 200)" required:"false"`
}

type ListSandboxJobsOutput struct {
	Body struct {
		Jobs []models.SandboxJob `json:"jobs"`
	}
}

type GetSandboxJobOutput struct {
	Body models.SandboxJob
}

type GetSandboxJobLogsInput struct {
	OrganizationID string `path:"organization_id"`
	JobID          string `path:"job_id"`
	Cursor         int64  `query:"cursor"`
	Limit          int    `query:"limit"`
}

type GetSandboxJobLogsOutput struct {
	Body struct {
		Logs       []models.SandboxJobLog `json:"logs"`
		NextCursor int64                  `json:"next_cursor"`
	}
}

type CancelSandboxJobInput struct {
	OrganizationID string `path:"organization_id"`
	JobID          string `path:"job_id"`
}
