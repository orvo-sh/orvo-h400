-- name: CreateSession :one
INSERT INTO
    sessions (
        id,
        token,
        user_id,
        active_organization_id,
        ip_address,
        user_agent,
        expires_at,
        created_at,
        updated_at
    )
VALUES (
        $1,
        $2,
        $3,
        $4,
        $5,
        $6,
        $7,
        NOW(),
        NOW()
    )
RETURNING
    *;

-- name: GetSessionByToken :one
SELECT * FROM sessions WHERE token = $1 AND expires_at > NOW();

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: GetSessionsByUserID :many
SELECT * FROM sessions WHERE user_id = $1 AND expires_at > NOW();

-- name: UpdateSessionExpiry :one
UPDATE sessions
SET
    expires_at = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: SetActiveOrganization :one
UPDATE sessions
SET
    active_organization_id = $2,
    updated_at = NOW()
WHERE
    id = $1
RETURNING
    *;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionByToken :exec
DELETE FROM sessions WHERE token = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= NOW();

-- name: GetSessionWithUser :one
SELECT
    s.id as session_id,
    s.token,
    s.user_id,
    s.active_organization_id,
    s.ip_address,
    s.user_agent,
    s.expires_at,
    s.created_at as session_created_at,
    s.updated_at as session_updated_at,
    u.id as user_id,
    u.email,
    u.email_verified,
    u.name,
    u.image,
    u.created_at as user_created_at,
    u.updated_at as user_updated_at
FROM sessions s
    JOIN users u ON s.user_id = u.id
WHERE
    s.token = $1
    AND s.expires_at > NOW();

-- name: GetSessionWithUserAndOrganization :one
SELECT
    s.id as session_id,
    s.token,
    s.user_id,
    s.active_organization_id,
    s.ip_address,
    s.user_agent,
    s.expires_at,
    s.created_at as session_created_at,
    s.updated_at as session_updated_at,
    u.id as user_id,
    u.email,
    u.email_verified,
    u.name as user_name,
    u.image as user_image,
    u.created_at as user_created_at,
    u.updated_at as user_updated_at,
    o.id as org_id,
    o.name as org_name,
    o.slug as org_slug,
    o.logo as org_logo,
    o.metadata as org_metadata,
    o.created_at as org_created_at,
    o.updated_at as org_updated_at,
    om.role as member_role
FROM
    sessions s
    JOIN users u ON s.user_id = u.id
    LEFT JOIN organizations o ON s.active_organization_id = o.id
    LEFT JOIN organization_members om ON o.id = om.organization_id
    AND u.id = om.user_id
WHERE
    s.token = $1
    AND s.expires_at > NOW();