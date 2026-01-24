-- name: CreateOrganizationMember :one
INSERT INTO organization_members (id, organization_id, user_id, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
RETURNING *;

-- name: GetMemberByOrgAndUser :one
SELECT * FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: GetMemberByID :one
SELECT * FROM organization_members WHERE id = $1;

-- name: ListMembersByOrganizationID :many
SELECT 
    om.*,
    u.email,
    u.name,
    u.image
FROM organization_members om
JOIN users u ON om.user_id = u.id
WHERE om.organization_id = $1
ORDER BY om.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMembersByOrganizationID :one
SELECT COUNT(*) FROM organization_members WHERE organization_id = $1;

-- name: UpdateMemberRole :one
UPDATE organization_members
SET role = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteMember :exec
DELETE FROM organization_members WHERE id = $1;

-- name: DeleteMemberByOrgAndUser :exec
DELETE FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: GetUserOrganizationMemberships :many
SELECT 
    om.*,
    o.name as org_name,
    o.slug as org_slug,
    o.logo as org_logo
FROM organization_members om
JOIN organizations o ON om.organization_id = o.id
WHERE om.user_id = $1
ORDER BY om.created_at DESC;
