// Package todo provides todo item repository.
package todo

import (
	"context"

	"github.com/google/uuid"
	dbutil "github.com/your-org/go-backend-template/internal/db/dbutil"
	db "github.com/your-org/go-backend-template/internal/db/sqlc"
)

// Repository provides database access for todos. All methods return
// feature-local row DTOs; the service never imports internal/db/sqlc
// or pgtype.
type Repository struct {
	db *db.Queries
}

// NewRepository creates a new todo repository.
func NewRepository(q *db.Queries) *Repository {
	return &Repository{db: q}
}

// GetTodoByID gets a todo by ID.
func (r *Repository) GetTodoByID(ctx context.Context, id uuid.UUID) (*TodoRow, error) {
	t, err := r.db.GetTodoByID(ctx, dbutil.UUIDToPgtypeValue(id))
	if err != nil {
		return nil, err
	}
	return todoRowFromDB(&t), nil
}

// ListTodosByUserID lists all todos for a user.
func (r *Repository) ListTodosByUserID(ctx context.Context, userID uuid.UUID) ([]TodoRow, error) {
	rows, err := r.db.ListTodosByUserID(ctx, dbutil.UUIDToPgtypeValue(userID))
	if err != nil {
		return nil, err
	}
	return todosFromDB(rows), nil
}

// CreateTodo creates a new todo.
func (r *Repository) CreateTodo(ctx context.Context, in TodoCreateInput) (*TodoRow, error) {
	t, err := r.db.CreateTodo(ctx, in.toCreateParams())
	if err != nil {
		return nil, err
	}
	return todoRowFromDB(&t), nil
}

// UpdateTodoPartial applies a PATCH-style update. nil pointer fields
// preserve the existing column.
func (r *Repository) UpdateTodoPartial(ctx context.Context, in TodoUpdateInput) (TodoRow, error) {
	updated, err := r.db.UpdateTodoPartial(ctx, in.toUpdateParamsPartial())
	if err != nil {
		return TodoRow{}, err
	}
	return *todoRowFromDB(&updated), nil
}

// DeleteTodo deletes a todo.
func (r *Repository) DeleteTodo(ctx context.Context, id uuid.UUID) error {
	return r.db.DeleteTodo(ctx, dbutil.UUIDToPgtypeValue(id))
}
