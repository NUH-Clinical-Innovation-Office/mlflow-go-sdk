-- Create users table
-- Stores user accounts with authentication details
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    approved_user_id UUID NOT NULL REFERENCES approved_users(id) ON DELETE CASCADE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index on email for fast lookups during login
CREATE INDEX idx_users_email ON users(email);

-- Create index on approved_user_id for JOIN queries
CREATE INDEX idx_users_approved_user_id ON users(approved_user_id);

-- Create index on is_active for filtering active users
CREATE INDEX idx_users_is_active ON users(is_active);
