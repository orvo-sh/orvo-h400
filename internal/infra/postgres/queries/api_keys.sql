-- name: CreateApiKey :one
INSERT INTO api_keys (id, organization_id, key_hash, name, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING *;

-- name: GetApiKeyByID :one
SELECT * FROM api_keys WHERE id = $1;

-- name: GetApiKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL
AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListApiKeysByOrganizationID :many
SELECT * FROM api_keys
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: RevokeApiKey :one
UPDATE api_keys
SET revoked_at = NOW()
WHERE id = $1 AND organization_id = $2 AND revoked_at IS NULL
RETURNING *;

-- name: UpdateApiKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = NOW()
WHERE key_hash = $1;

-- name: DeleteApiKey :exec
DELETE FROM api_keys WHERE id = $1 AND organization_id = $2;
