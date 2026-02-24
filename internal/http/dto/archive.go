package dto

import "github.com/orvo-sh/orvo/internal/domain/models"

type CreateRestoreJobInput struct {
	OrganizationID string `path:"organization_id"`
	Body           struct {
		Signal   string `json:"signal" doc:"Signal name: logs, traces, metrics" required:"true"`
		StartDay string `json:"start_day" doc:"Start day in YYYY-MM-DD" required:"true"`
		EndDay   string `json:"end_day" doc:"End day in YYYY-MM-DD" required:"true"`
	}
}

type CreateRestoreJobOutput struct {
	Body models.RestoreJob
}

type GetRestoreJobInput struct {
	OrganizationID string `path:"organization_id"`
	RestoreJobID   string `path:"restore_job_id"`
}

type GetRestoreJobOutput struct {
	Body models.RestoreJob
}
