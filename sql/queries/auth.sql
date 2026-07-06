-- name: GetUserByEmail :one
SELECT id, approved_user_id, email, password_hash, is_active, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, approved_user_id, email, password_hash, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (approved_user_id, email, password_hash, is_active)
VALUES ($1, $2, $3, $4)
RETURNING id, approved_user_id, email, password_hash, is_active, created_at, updated_at;

-- name: UpdateUser :one
UPDATE users
SET email = $2, password_hash = $3, is_active = $4, updated_at = NOW()
WHERE id = $1
RETURNING id, approved_user_id, email, password_hash, is_active, created_at, updated_at;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: GetUserRoles :many
SELECT r.id, r.name, r.description, r.created_at
FROM roles r
INNER JOIN user_roles ur ON r.id = ur.role_id
WHERE ur.user_id = $1
ORDER BY r.name;

-- name: GetUserWithRolesAndApproved :one
-- Single-query aggregate used by the auth middleware on every request.
-- Returns the user row, a text[] of role names, and the joined approved_user
-- (NULL when the user has no approved_user link). Three roundtrips in one.
SELECT
    u.id, u.approved_user_id, u.email, u.password_hash, u.is_active, u.created_at, u.updated_at,
    COALESCE(
        (SELECT array_agg(r.name ORDER BY r.name)
         FROM user_roles ur JOIN roles r ON r.id = ur.role_id
         WHERE ur.user_id = u.id),
        ARRAY[]::text[]
    ) AS role_names,
    a.id          AS approved_id,
    a.email       AS approved_email,
    a.first_name  AS approved_first_name,
    a.created_by  AS approved_created_by,
    a.created_at  AS approved_created_at,
    a.updated_at  AS approved_updated_at
FROM users u
LEFT JOIN approved_users a ON a.id = u.approved_user_id
WHERE u.id = $1;

-- name: GetRolesByNames :many
SELECT id, name, description, created_at
FROM roles
WHERE name = ANY($1::TEXT[]);

-- name: GetRoleByName :one
SELECT id, name, description, created_at
FROM roles
WHERE name = $1
LIMIT 1;

-- name: ApprovedUserExists :one
SELECT EXISTS(SELECT 1 FROM approved_users WHERE id = $1);

-- name: AssignRole :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: RemoveRoleFromUser :exec
DELETE FROM user_roles
WHERE user_id = $1 AND role_id = $2;
