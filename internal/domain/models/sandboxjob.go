package models

import "time"

type SandboxJobState string

const (
	SandboxJobStateQueued    SandboxJobState = "queued"
	SandboxJobStateRunning   SandboxJobState = "running"
	SandboxJobStateSucceeded SandboxJobState = "succeeded"
	SandboxJobStateFailed    SandboxJobState = "failed"
	SandboxJobStateCancelled SandboxJobState = "cancelled"
	SandboxJobStateTimedOut  SandboxJobState = "timed_out"
)

type SandboxJob struct {
	ID                string          `json:"id"`
	OrganizationID    string          `json:"organization_id"`
	RepositoryID      string          `json:"repository_id"`
	RequestedByUserID string          `json:"requested_by_user_id"`
	Mode              string          `json:"mode"`
	State             SandboxJobState `json:"state"`
	RuntimeType       string          `json:"runtime_type"`
	SandboxInstanceID string          `json:"sandbox_instance_id"`
	TaskTitle         string          `json:"task_title"`
	CommitMessage     string          `json:"commit_message"`
	BaseBranch        string          `json:"base_branch"`
	IncidentContext   string          `json:"-"`
	IncidentPrompt    string          `json:"-"`
	BranchName        string          `json:"branch_name"`
	DraftPR           bool            `json:"draft_pr"`
	PullRequestNumber *int64          `json:"pull_request_number"`
	PullRequestURL    *string         `json:"pull_request_url"`
	CancelRequested   bool            `json:"cancel_requested"`
	Error             string          `json:"error,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	FinishedAt        *time.Time      `json:"finished_at,omitempty"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type SandboxJobLog struct {
	Seq       int64     `json:"seq"`
	Stream    string    `json:"stream"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type SandboxJobCommand struct {
	ID         string     `json:"id"`
	Ordinal    int        `json:"ordinal"`
	Command    string     `json:"command"`
	ExitCode   *int       `json:"exit_code"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}
