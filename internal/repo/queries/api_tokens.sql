-- name: CreateAPIToken :one
INSERT INTO api_tokens (
    user_id,
    name,
    token_hash,
    source,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetAPITokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = $1;

-- name: GetAPITokenByUUID :one
SELECT * FROM api_tokens WHERE uuid = $1;

-- name: ListAPITokensByUser :many
SELECT api_tokens.*
FROM api_tokens
JOIN users ON users.id = api_tokens.user_id
WHERE users.uuid = @user_uuid
ORDER BY api_tokens.created_at DESC;

-- name: RevokeAPITokenForUser :execrows
UPDATE api_tokens
SET revoked_at = now()
FROM users
WHERE api_tokens.user_id = users.id
  AND api_tokens.uuid = @token_uuid
  AND users.uuid = @user_uuid
  AND api_tokens.revoked_at IS NULL;

-- name: TouchAPITokenLastUsed :exec
UPDATE api_tokens
SET last_used_at = now()
WHERE id = @id
  AND (last_used_at IS NULL OR last_used_at < @threshold);

-- name: DeleteExpiredAPITokens :execrows
DELETE FROM api_tokens
WHERE expires_at IS NOT NULL AND expires_at < now();
