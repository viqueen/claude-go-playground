-- name: GetContent :one
SELECT * FROM collaboration.content WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ListContent :many
SELECT * FROM collaboration.content
WHERE deleted_at IS NULL
ORDER BY created_at LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountContent :one
SELECT count(*) FROM collaboration.content WHERE deleted_at IS NULL;

-- name: ListContentBySpace :many
SELECT * FROM collaboration.content
WHERE space_id = sqlc.arg('space_id') AND deleted_at IS NULL
ORDER BY created_at LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountContentBySpace :one
SELECT count(*) FROM collaboration.content
WHERE space_id = sqlc.arg('space_id') AND deleted_at IS NULL;

-- name: CreateContent :one
INSERT INTO collaboration.content (space_id, title, body, status, tags)
VALUES (sqlc.arg('space_id'), sqlc.arg('title'), sqlc.arg('body'), sqlc.arg('status'), sqlc.arg('tags'))
RETURNING *;

-- name: UpdateContent :one
UPDATE collaboration.content
SET title = COALESCE(sqlc.narg('title'), title),
    body = COALESCE(sqlc.narg('body'), body),
    status = COALESCE(sqlc.narg('status'), status),
    tags = COALESCE(sqlc.narg('tags'), tags),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteContentBySpace :exec
UPDATE collaboration.content
SET deleted_at = now(), updated_at = now()
WHERE space_id = sqlc.arg('space_id') AND deleted_at IS NULL;

-- name: SoftDeleteContent :one
UPDATE collaboration.content
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: RestoreContent :one
UPDATE collaboration.content
SET deleted_at = NULL, updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NOT NULL
RETURNING *;
