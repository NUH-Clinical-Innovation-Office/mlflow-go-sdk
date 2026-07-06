-- Seed data for integration tests

-- Insert test approved users first (required for users.approved_user_id FK)
INSERT INTO approved_users (id, email, first_name) VALUES
    ('00000000-0000-0000-0000-000000000030'::uuid, 'admin@example.com', 'Admin'),
    ('00000000-0000-0000-0000-000000000031'::uuid, 'user@example.com', 'Test')
ON CONFLICT (id) DO NOTHING;

-- Insert test roles
INSERT INTO roles (name, description) VALUES
    ('admin', 'Administrator with full access'),
    ('user', 'Regular user with limited access')
ON CONFLICT (name) DO NOTHING;

-- Insert test users (password: 'password123' hashed with bcrypt cost 4)
-- Hash generated using: bcrypt.HashCost4("password123")
INSERT INTO users (approved_user_id, email, password_hash, is_active) VALUES
    ('00000000-0000-0000-0000-000000000030'::uuid, 'admin@example.com', '$2a$04$KIXjSImZvPjCMtH8jQU5W.q8Q9C6vZJ9xVxK8yE3qL1nR5oP7mZ2G', true),
    ('00000000-0000-0000-0000-000000000031'::uuid, 'user@example.com', '$2a$04$KIXjSImZvPjCMtH8jQU5W.q8Q9C6vZJ9xVxK8yE3qL1nR5oP7mZ2G', true)
ON CONFLICT (email) DO NOTHING;

-- Assign roles to users
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'admin@example.com' AND r.name = 'admin'
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'user@example.com' AND r.name = 'user'
ON CONFLICT (user_id, role_id) DO NOTHING;

-- Insert sample todos for testing
INSERT INTO todos (user_id, title, description, is_completed, created_at, updated_at)
SELECT
    u.id,
    'Sample Todo 1',
    'This is a sample todo item for testing',
    false,
    NOW(),
    NOW()
FROM users u WHERE u.email = 'user@example.com'
ON CONFLICT DO NOTHING;

INSERT INTO todos (user_id, title, description, is_completed, created_at, updated_at)
SELECT
    u.id,
    'Sample Todo 2',
    'Another sample todo for integration tests',
    true,
    NOW(),
    NOW()
FROM users u WHERE u.email = 'user@example.com'
ON CONFLICT DO NOTHING;
