// Package todo provides todo item types.
package todo

import (
	"time"

	"github.com/google/uuid"
)

// TodoRow is the persistence-shape view of a row in the todos table.
// The service operates on this struct; it does not import
// internal/db/sqlc or pgtype.
type TodoRow struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Title       string
	Description *string
	IsCompleted bool
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TodoCreateInput is what the service hands to the repository to
// create a new todo. The repository fills ID and timestamps.
type TodoCreateInput struct {
	UserID      uuid.UUID
	Title       string
	Description *string
	IsCompleted bool
	DueDate     *time.Time
}

// TodoUpdateInput is what the service hands to the repository to
// update a todo. ID identifies the row; nil pointer fields preserve
// the existing column (PATCH semantics). Non-nil pointers overwrite.
type TodoUpdateInput struct {
	ID          uuid.UUID
	Title       *string
	Description *string
	IsCompleted *bool
	DueDate     *time.Time
}
