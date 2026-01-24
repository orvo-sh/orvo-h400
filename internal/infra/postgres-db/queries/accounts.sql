-- name: CreateAccount :one
INSERT INTO accounts (id, user_id, provider, provider_account_id, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
RETURNING *;

-- name: GetAccountByProvider :one
SELECT * FROM accounts
WHERE provider = $1 AND provider_account_id = $2;

-- name: GetAccountsByUserID :many
SELECT * FROM accounts WHERE user_id = $1;

-- name: UpdateAccountPassword :exec
UPDATE accounts
SET password_hash = $2, updated_at = NOW()
WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = $1;
