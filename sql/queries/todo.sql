-- name: GetTodoByID :one
SELECT id, user_id, title, description, is_completed, due_date, created_at, updated_at
FROM todos
WHERE id = $1;

-- name: ListTodosByUserID :many
SELECT id, user_id, title, description, is_completed, due_date, created_at, updated_at
FROM todos
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListTodosByUserIDWithFilters :many
SELECT id, user_id, title, description, is_completed, due_date, created_at, updated_at
FROM todos
WHERE user_id = $1
    AND (sqlc.narg('is_completed')::BOOLEAN IS NULL OR is_completed = sqlc.narg('is_completed')::BOOLEAN)
ORDER BY
    CASE WHEN sqlc.arg('sort_by') = 'due_date' THEN due_date END,
    CASE WHEN sqlc.arg('sort_by') = 'created_at' THEN created_at END,
    CASE WHEN sqlc.arg('sort_by') = 'title' THEN title END
DESC NULLS LAST;

-- name: CreateTodo :one
INSERT INTO todos (user_id, title, description, is_completed, due_date)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, title, description, is_completed, due_date, created_at, updated_at;

-- name: UpdateTodo :one
UPDATE todos
SET title = $2, description = $3, is_completed = $4, due_date = $5, updated_at = NOW()
WHERE id = $1
RETURNING id, user_id, title, description, is_completed, due_date, created_at, updated_at;

-- name: UpdateTodoPartial :one
-- PATCH-style update. Any nil named arg preserves the existing column.
-- The service layer is responsible for converting empty strings to nil.
UPDATE todos
SET
    title        = COALESCE(sqlc.narg('title')::TEXT,         title),
    description  = COALESCE(sqlc.narg('description')::TEXT,   description),
    is_completed = COALESCE(sqlc.narg('is_completed')::BOOLEAN, is_completed),
    due_date     = COALESCE(sqlc.narg('due_date')::TIMESTAMPTZ, due_date),
    updated_at   = NOW()
WHERE id = sqlc.arg('id')
RETURNING id, user_id, title, description, is_completed, due_date, created_at, updated_at;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = $1;

-- name: DeleteTodosByUserID :exec
DELETE FROM todos
WHERE user_id = $1;
