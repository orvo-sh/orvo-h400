package models

import "time"

type RestoreJob struct {
	ID               string           `json:"id"`
	OrganizationID   string           `json:"organization_id"`
	Signal           string           `json:"signal"`
	StartDay         time.Time        `json:"start_day"`
	EndDay           time.Time        `json:"end_day"`
	State            string           `json:"state"`
	RequestedBy      string           `json:"requested_by,omitempty"`
	TotalItems       int              `json:"total_items"`
	CompletedItems   int              `json:"completed_items"`
	TotalBytes       int64            `json:"total_bytes"`
	DoneBytes        int64            `json:"done_bytes"`
	EstimatedSeconds int              `json:"estimated_seconds"`
	Error            string           `json:"error,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	StartedAt        *time.Time       `json:"started_at,omitempty"`
	FinishedAt       *time.Time       `json:"finished_at,omitempty"`
	Items            []RestoreJobItem `json:"items,omitempty"`
}

type RestoreJobItem struct {
	ID              string     `json:"id"`
	Day             time.Time  `json:"day"`
	ObjectKey       string     `json:"object_key"`
	State           string     `json:"state"`
	ObjectSizeBytes int64      `json:"object_size_bytes"`
	RestoredRows    int64      `json:"restored_rows"`
	Error           string     `json:"error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}
