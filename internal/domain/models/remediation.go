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

type AutoResolveRepositoryContext struct {
	ServiceName        string `json:"service_name"`
	RepositoryID       string `json:"repository_id"`
	RepositoryFullName string `json:"repository_full_name"`
	Reason             string `json:"reason"`
}

type AutoResolveContextSummary struct {
	TraceSpanCount      int `json:"trace_span_count"`
	NearbyErrorLogCount int `json:"nearby_error_log_count"`
}

type AutoResolvePreview struct {
	LogID               string                         `json:"log_id"`
	ServiceName         string                         `json:"service_name"`
	RepositoryID        string                         `json:"repository_id"`
	RepositoryFullName  string                         `json:"repository_full_name"`
	BaseBranch          string                         `json:"base_branch"`
	TaskTitle           string                         `json:"task_title"`
	CommitMessage       string                         `json:"commit_message"`
	ValidationCommands  []string                       `json:"validation_commands"`
	RelatedRepositories []AutoResolveRepositoryContext `json:"related_repositories"`
	ContextSummary      AutoResolveContextSummary      `json:"context_summary"`
}

type AutoResolveThreshold struct {
	ID                    string     `json:"id"`
	OrganizationID        string     `json:"organization_id"`
	ServiceName           string     `json:"service_name"`
	MetricName            string     `json:"metric_name"`
	ThresholdValue        float64    `json:"threshold_value"`
	LookbackWindowSeconds int        `json:"lookback_window_seconds"`
	CooldownSeconds       int        `json:"cooldown_seconds"`
	Quorum                int        `json:"quorum"`
	Enabled               bool       `json:"enabled"`
	LastTriggeredAt       *time.Time `json:"last_triggered_at"`
	CreatedByUserID       string     `json:"created_by_user_id"`
	UpdatedByUserID       string     `json:"updated_by_user_id"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
