-- name: CreateDashboard :one
INSERT INTO dashboards (id, organization_id, name, description, panels, layout, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
RETURNING *;

-- name: GetDashboardByID :one
SELECT * FROM dashboards
WHERE id = $1 AND organization_id = $2;

-- name: ListDashboardsByOrganizationID :many
SELECT * FROM dashboards
WHERE organization_id = $1
ORDER BY updated_at DESC;

-- name: UpdateDashboard :one
UPDATE dashboards
SET name = $3, description = $4, panels = $5, layout = $6, updated_at = NOW()
WHERE id = $1 AND organization_id = $2
RETURNING *;

-- name: DeleteDashboard :exec
DELETE FROM dashboards WHERE id = $1 AND organization_id = $2;
