-- Drop seed data (for testing purposes)

-- Delete sample todos
DELETE FROM todos WHERE title IN ('Sample Todo 1', 'Sample Todo 2');

-- Delete user role assignments
DELETE FROM user_roles WHERE user_id IN (
    SELECT id FROM users WHERE email IN ('admin@example.com', 'user@example.com')
);

-- Delete test users
DELETE FROM users WHERE email IN ('admin@example.com', 'user@example.com');

-- Delete test roles
DELETE FROM roles WHERE name IN ('admin', 'user');
