// Package todo provides todo item service.
package todo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/go-backend-template/internal/domain"
)

var (
	ErrTodoNotFound      = errors.New("todo not found")
	ErrTodoNotOwned      = errors.New("todo does not belong to user")
	ErrInvalidTodoParams = errors.New("invalid todo parameters")
)

// Service provides todo business logic
type Service struct {
	repo TodoRepository
}

// NewService creates a new todo service
func NewService(repo TodoRepository) *Service {
	return &Service{repo: repo}
}

// ListByUserID lists all todos for a user
func (s *Service) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Todo, error) {
	rows, err := s.repo.ListTodosByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Todo, len(rows))
	for i := range rows {
		out[i] = rowToDomain(&rows[i])
	}
	return out, nil
}

// GetByID gets a todo by ID, ensuring it belongs to the user
func (s *Service) GetByID(ctx context.Context, todoID, userID uuid.UUID) (*domain.Todo, error) {
	row, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		return nil, ErrTodoNotFound
	}
	if row.UserID != userID {
		return nil, ErrTodoNotOwned
	}
	d := rowToDomain(row)
	return &d, nil
}

// Create creates a new todo
func (s *Service) Create(ctx context.Context, userID uuid.UUID, title string, description *string, dueDate *time.Time) (*domain.Todo, error) {
	if title == "" {
		return nil, ErrInvalidTodoParams
	}

	row, err := s.repo.CreateTodo(ctx, TodoCreateInput{
		UserID:      userID,
		Title:       title,
		Description: description,
		IsCompleted: false,
		DueDate:     dueDate,
	})
	if err != nil {
		return nil, err
	}
	d := rowToDomain(row)
	return &d, nil
}

// Update applies a PATCH-style update to a todo, ensuring it belongs to
// the user. Any nil pointer field preserves the existing column; non-nil
// overwrites. A nil title preserves the existing title; an explicit
// empty-string title is rejected as invalid.
func (s *Service) Update(ctx context.Context, todoID, userID uuid.UUID, title, description *string, isCompleted *bool, dueDate *time.Time) (*domain.Todo, error) {
	if title != nil && *title == "" {
		return nil, ErrInvalidTodoParams
	}

	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		return nil, ErrTodoNotFound
	}
	if existing.UserID != userID {
		return nil, ErrTodoNotOwned
	}

	row, err := s.repo.UpdateTodoPartial(ctx, TodoUpdateInput{
		ID:          todoID,
		Title:       title,
		Description: description,
		IsCompleted: isCompleted,
		DueDate:     dueDate,
	})
	if err != nil {
		return nil, err
	}
	d := rowToDomain(&row)
	return &d, nil
}

// Delete deletes a todo, ensuring it belongs to the user
func (s *Service) Delete(ctx context.Context, todoID, userID uuid.UUID) error {
	existing, err := s.repo.GetTodoByID(ctx, todoID)
	if err != nil {
		return ErrTodoNotFound
	}
	if existing.UserID != userID {
		return ErrTodoNotOwned
	}
	return s.repo.DeleteTodo(ctx, todoID)
}

// rowToDomain converts a feature row DTO into the cross-feature domain
// type. Kept private; callers do not need the boundary explained.
func rowToDomain(r *TodoRow) domain.Todo {
	return domain.Todo{
		ID:          r.ID,
		UserID:      r.UserID,
		Title:       r.Title,
		Description: r.Description,
		IsCompleted: r.IsCompleted,
		DueDate:     r.DueDate,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
