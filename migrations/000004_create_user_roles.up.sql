-- Create user_roles junction table
-- Maps users to their assigned roles (many-to-many)
CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

-- Create index on role_id for reverse lookups
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
