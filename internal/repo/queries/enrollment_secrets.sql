-- name: CreateEnrollmentSecret :one
INSERT INTO enrollment_secrets (
    secret_id,
    type,
    owner_user_uuid,
    created_by
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetActiveAnonymousEnrollmentSecret :one
SELECT * FROM enrollment_secrets
WHERE type = 'anonymous' AND revoked_at IS NULL
LIMIT 1;

-- name: GetActiveOwnedEnrollmentSecret :one
SELECT * FROM enrollment_secrets
WHERE type = 'owned' AND owner_user_uuid = $1 AND revoked_at IS NULL
LIMIT 1;

-- name: GetEnrollmentSecretBySecretID :one
SELECT * FROM enrollment_secrets WHERE secret_id = $1;

-- name: GetEnrollmentSecretByUUID :one
SELECT * FROM enrollment_secrets WHERE uuid = $1;

-- name: RevokeEnrollmentSecretByUUID :execrows
UPDATE enrollment_secrets
SET revoked_at = now(), revoked_by = @revoked_by
WHERE uuid = @uuid AND revoked_at IS NULL;

-- name: RevokeEnrollmentSecretByID :execrows
UPDATE enrollment_secrets
SET revoked_at = now(), revoked_by = @revoked_by
WHERE id = @id AND revoked_at IS NULL;

-- name: ListEnrollmentSecrets :many
SELECT
    enrollment_secrets.*,
    COALESCE(NULLIF(generator.email, ''), NULLIF(generator.name, ''), generator.username, '') AS generated_by,
    (SELECT count(*) FROM nodes WHERE nodes.enrollment_secret_id = enrollment_secrets.id) AS machine_count
FROM enrollment_secrets
LEFT JOIN users AS generator ON generator.id = enrollment_secrets.created_by
ORDER BY enrollment_secrets.created_at DESC;

-- name: DeleteUnusedOwnedEnrollmentSecrets :execrows
DELETE FROM enrollment_secrets
WHERE type = 'owned'
  AND created_at < @cutoff::timestamptz
  AND NOT EXISTS (
    SELECT 1 FROM nodes WHERE nodes.enrollment_secret_id = enrollment_secrets.id
  );

-- name: ListNodesByEnrollmentSecretID :many
SELECT nodes.* FROM nodes
JOIN enrollment_secrets ON enrollment_secrets.id = nodes.enrollment_secret_id
WHERE enrollment_secrets.uuid = $1
ORDER BY nodes.created_at DESC;
