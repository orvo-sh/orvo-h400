-- name: CreateInvitation :one
INSERT INTO organization_invitations (id, organization_id, email, role, invited_by_id, status, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, 'pending', $6, NOW(), NOW())
RETURNING *;

-- name: GetInvitationByID :one
SELECT * FROM organization_invitations WHERE id = $1;

-- name: GetPendingInvitationByEmailAndOrg :one
SELECT * FROM organization_invitations
WHERE email = $1 AND organization_id = $2 AND status = 'pending' AND expires_at > NOW();

-- name: ListInvitationsByOrganizationID :many
SELECT 
    oi.*,
    u.name as inviter_name,
    u.email as inviter_email
FROM organization_invitations oi
JOIN users u ON oi.invited_by_id = u.id
WHERE oi.organization_id = $1
ORDER BY oi.created_at DESC;

-- name: ListPendingInvitationsByEmail :many
SELECT 
    oi.*,
    o.name as org_name,
    o.slug as org_slug,
    u.name as inviter_name
FROM organization_invitations oi
JOIN organizations o ON oi.organization_id = o.id
JOIN users u ON oi.invited_by_id = u.id
WHERE oi.email = $1 AND oi.status = 'pending' AND oi.expires_at > NOW()
ORDER BY oi.created_at DESC;

-- name: UpdateInvitationStatus :one
UPDATE organization_invitations
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteInvitation :exec
DELETE FROM organization_invitations WHERE id = $1;

-- name: DeleteExpiredInvitations :exec
DELETE FROM organization_invitations WHERE expires_at <= NOW() AND status = 'pending';
