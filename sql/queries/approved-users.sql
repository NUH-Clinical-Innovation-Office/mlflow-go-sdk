-- name: GetApprovedUserByEmail :one
SELECT id, email, first_name, created_by, created_at, updated_at
FROM approved_users
WHERE email = $1;

-- name: GetApprovedUserByID :one
SELECT id, email, first_name, created_by, created_at, updated_at
FROM approved_users
WHERE id = $1;

-- name: ListApprovedUsers :many
SELECT id, email, first_name, created_by, created_at, updated_at
FROM approved_users
ORDER BY created_at DESC;

-- name: CreateApprovedUser :one
INSERT INTO approved_users (email, first_name, created_by)
VALUES ($1, $2, $3)
RETURNING id, email, first_name, created_by, created_at, updated_at;

-- name: CreateApprovedUsersBulk :many
INSERT INTO approved_users (email, first_name, created_by)
SELECT unnest($1::TEXT[]) AS email,
       unnest($2::TEXT[]) AS first_name,
       unnest($3::UUID[]) AS created_by
RETURNING id, email, first_name, created_by, created_at, updated_at;

-- name: DeleteApprovedUser :execrows
DELETE FROM approved_users
WHERE id = $1;
