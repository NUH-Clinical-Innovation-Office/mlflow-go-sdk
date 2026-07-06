// Package todo provides todo interfaces for dependency injection.
package todo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/go-backend-template/internal/domain"
)

// TodoRepository defines the interface for todo data access. Methods
// return feature-local row DTOs; the interface has no dependency on
// internal/db/sqlc or pgtype.
type TodoRepository interface {
	GetTodoByID(ctx context.Context, id uuid.UUID) (*TodoRow, error)
	ListTodosByUserID(ctx context.Context, userID uuid.UUID) ([]TodoRow, error)
	CreateTodo(ctx context.Context, in TodoCreateInput) (*TodoRow, error)
	UpdateTodoPartial(ctx context.Context, in TodoUpdateInput) (TodoRow, error)
	DeleteTodo(ctx context.Context, id uuid.UUID) error
}

// TodoService defines the interface for todo business logic.
type TodoService interface {
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Todo, error)
	GetByID(ctx context.Context, todoID, userID uuid.UUID) (*domain.Todo, error)
	Create(ctx context.Context, userID uuid.UUID, title string, description *string, dueDate *time.Time) (*domain.Todo, error)
	Update(ctx context.Context, todoID, userID uuid.UUID, title *string, description *string, isCompleted *bool, dueDate *time.Time) (*domain.Todo, error)
	Delete(ctx context.Context, todoID, userID uuid.UUID) error
}
