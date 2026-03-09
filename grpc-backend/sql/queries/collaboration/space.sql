-- name: GetSpace :one
SELECT * FROM collaboration.space WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ListSpaces :many
SELECT * FROM collaboration.space
WHERE deleted_at IS NULL
ORDER BY created_at LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountSpaces :one
SELECT count(*) FROM collaboration.space WHERE deleted_at IS NULL;

-- name: CreateSpace :one
INSERT INTO collaboration.space (name, key, description, status, visibility)
VALUES (sqlc.arg('name'), sqlc.arg('key'), sqlc.arg('description'), sqlc.arg('status'), sqlc.arg('visibility'))
RETURNING *;

-- name: UpdateSpace :one
UPDATE collaboration.space
SET name = COALESCE(sqlc.narg('name'), name),
    key = COALESCE(sqlc.narg('key'), key),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('status'), status),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteSpace :one
UPDATE collaboration.space
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: RestoreSpace :one
UPDATE collaboration.space
SET deleted_at = NULL, updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NOT NULL
RETURNING *;
