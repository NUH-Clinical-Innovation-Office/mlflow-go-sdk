package todo

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	dbutil "github.com/your-org/go-backend-template/internal/db/dbutil"
	db "github.com/your-org/go-backend-template/internal/db/sqlc"
)

// todoRowFromDB converts a sqlc-generated todo row into the feature
// row DTO.
func todoRowFromDB(t *db.Todo) *TodoRow {
	if t == nil {
		return nil
	}
	return &TodoRow{
		ID:          dbutil.PgUUIDToValue(t.ID),
		UserID:      dbutil.PgUUIDToValue(t.UserID),
		Title:       t.Title,
		Description: t.Description,
		IsCompleted: t.IsCompleted,
		DueDate:     dbutil.PgTimestamptzToPtr(t.DueDate),
		CreatedAt:   t.CreatedAt.Time,
		UpdatedAt:   t.UpdatedAt.Time,
	}
}

// todosFromDB converts a slice of sqlc todo rows.
func todosFromDB(ts []db.Todo) []TodoRow {
	out := make([]TodoRow, len(ts))
	for i := range ts {
		t := ts[i]
		out[i] = *todoRowFromDB(&t)
	}
	return out
}

// toCreateParams builds the sqlc param struct for CreateTodo.
func (in TodoCreateInput) toCreateParams() db.CreateTodoParams {
	return db.CreateTodoParams{
		UserID:      dbutil.UUIDToPgtypeValue(in.UserID),
		Title:       in.Title,
		Description: in.Description,
		IsCompleted: in.IsCompleted,
		DueDate:     dueDateToPg(in.DueDate),
	}
}

// toUpdateParamsPartial builds the sqlc param struct for UpdateTodoPartial.
// nil pointer fields become NULL in the named-arg form, and the COALESCE
// in the SQL preserves the existing column.
func (in TodoUpdateInput) toUpdateParamsPartial() db.UpdateTodoPartialParams {
	return db.UpdateTodoPartialParams{
		ID:          dbutil.UUIDToPgtypeValue(in.ID),
		Title:       in.Title,
		Description: in.Description,
		IsCompleted: in.IsCompleted,
		DueDate:     dueDateToPg(in.DueDate),
	}
}

func dueDateToPg(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
