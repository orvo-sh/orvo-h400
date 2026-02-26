package models

import "time"

type ServiceRemediationMapping struct {
	ID                 string    `json:"id"`
	OrganizationID     string    `json:"organization_id"`
	ServiceName        string    `json:"service_name"`
	RepositoryID       string    `json:"repository_id"`
	RepositoryFullName string    `json:"repository_full_name"`
	CreatedByUserID    string    `json:"created_by_user_id"`
	UpdatedByUserID    string    `json:"updated_by_user_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AutoResolveContextSummary struct {
	TraceSpanCount      int `json:"trace_span_count"`
	NearbyErrorLogCount int `json:"nearby_error_log_count"`
}

type AutoResolvePreview struct {
	LogID              string                    `json:"log_id"`
	ServiceName        string                    `json:"service_name"`
	RepositoryID       string                    `json:"repository_id"`
	RepositoryFullName string                    `json:"repository_full_name"`
	BaseBranch         string                    `json:"base_branch"`
	TaskTitle          string                    `json:"task_title"`
	CommitMessage      string                    `json:"commit_message"`
	ValidationCommands []string                  `json:"validation_commands"`
	ContextSummary     AutoResolveContextSummary `json:"context_summary"`
}
