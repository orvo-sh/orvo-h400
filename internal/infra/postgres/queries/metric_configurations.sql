-- name: CreateMetricConfiguration :one
INSERT INTO metric_configurations (id, organization_id, metric_name, config, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
RETURNING *;

-- name: GetMetricConfiguration :one
SELECT * FROM metric_configurations
WHERE organization_id = $1 AND metric_name = $2;

-- name: ListMetricConfigurationsByOrganizationID :many
SELECT * FROM metric_configurations
WHERE organization_id = $1
ORDER BY metric_name ASC;

-- name: UpdateMetricConfiguration :one
UPDATE metric_configurations
SET config = $3, enabled = $4, updated_at = NOW()
WHERE organization_id = $1 AND metric_name = $2
RETURNING *;

-- name: DeleteMetricConfiguration :exec
DELETE FROM metric_configurations
WHERE organization_id = $1 AND metric_name = $2;
