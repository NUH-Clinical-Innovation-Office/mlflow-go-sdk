-- Create approved_users table
-- Stores users who are approved to create accounts
CREATE TABLE IF NOT EXISTS approved_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    created_by UUID REFERENCES approved_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index on email for fast lookups
CREATE INDEX idx_approved_users_email ON approved_users(email);

-- Create index on created_by for hierarchical queries
CREATE INDEX idx_approved_users_created_by ON approved_users(created_by);
