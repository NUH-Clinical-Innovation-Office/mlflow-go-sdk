-- Create todos table
-- Stores user-scoped todo items
CREATE TABLE IF NOT EXISTS todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    is_completed BOOLEAN NOT NULL DEFAULT false,
    due_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index on user_id for user-scoped queries
CREATE INDEX idx_todos_user_id ON todos(user_id);

-- Create index on is_completed for filtering
CREATE INDEX idx_todos_is_completed ON todos(is_completed);

-- Create index on due_date for sorting
CREATE INDEX idx_todos_due_date ON todos(due_date);

-- Create composite index for common query pattern
CREATE INDEX idx_todos_user_is_completed ON todos(user_id, is_completed);
