// Package todo provides todo item handlers.
package todo

import (
	"errors"
	"net/http"
	"time"

	apiTypes "github.com/oapi-codegen/runtime/types"
	"github.com/your-org/go-backend-template/internal/api"
	"github.com/your-org/go-backend-template/internal/domain"
	http2 "github.com/your-org/go-backend-template/internal/http"
	"github.com/your-org/go-backend-template/internal/middleware"
)

// TodoResponse represents a todo response
type TodoResponse struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	IsCompleted bool       `json:"is_completed"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Handler holds todo dependencies
type Handler struct {
	svc TodoService
}

// NewHandler creates a new todo handler
func NewHandler(svc TodoService) *Handler {
	return &Handler{
		svc: svc,
	}
}

// ListTodos implements api.ServerInterface.
func (h *Handler) ListTodos(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	todos, err := h.svc.ListByUserID(r.Context(), user.ID)
	if err != nil {
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := make([]TodoResponse, len(todos))
	for i := range todos {
		response[i] = toTodoResponse(&todos[i])
	}

	http2.RespondJSON(w, http.StatusOK, response)
}

// CreateTodo implements api.ServerInterface.
func (h *Handler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req api.CreateTodoRequest
	if err := http2.DecodeJSON(w, r, 1<<20, &req); err != nil {
		if errors.Is(err, http2.ErrBodyTooLarge) {
			http2.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		http2.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.svc.Create(r.Context(), user.ID, req.Title, req.Description, req.DueDate)
	if err != nil {
		if errors.Is(err, ErrInvalidTodoParams) {
			http2.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusCreated, toTodoResponse(todo))
}

// GetTodo implements api.ServerInterface.
func (h *Handler) GetTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	todo, err := h.svc.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, ErrTodoNotFound) || errors.Is(err, ErrTodoNotOwned) {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusOK, toTodoResponse(todo))
}

// UpdateTodo implements api.ServerInterface.
func (h *Handler) UpdateTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req api.UpdateTodoRequest
	if err := http2.DecodeJSON(w, r, 1<<20, &req); err != nil {
		if errors.Is(err, http2.ErrBodyTooLarge) {
			http2.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		http2.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.svc.Update(r.Context(), id, user.ID, req.Title, req.Description, req.IsCompleted, req.DueDate)
	if err != nil {
		if errors.Is(err, ErrTodoNotFound) || errors.Is(err, ErrTodoNotOwned) {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		if errors.Is(err, ErrInvalidTodoParams) {
			http2.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusOK, toTodoResponse(todo))
}

// DeleteTodo implements api.ServerInterface.
func (h *Handler) DeleteTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.svc.Delete(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, ErrTodoNotFound) || errors.Is(err, ErrTodoNotOwned) {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toTodoResponse(todo *domain.Todo) TodoResponse {
	return TodoResponse{
		ID:          todo.ID.String(),
		UserID:      todo.UserID.String(),
		Title:       todo.Title,
		Description: todo.Description,
		IsCompleted: todo.IsCompleted,
		DueDate:     todo.DueDate,
		CreatedAt:   todo.CreatedAt,
		UpdatedAt:   todo.UpdatedAt,
	}
}
