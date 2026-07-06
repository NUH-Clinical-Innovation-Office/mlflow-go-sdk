// Package auth provides authentication handlers.
package auth

import (
	"errors"
	"net/http"
	"time"

	apiTypes "github.com/oapi-codegen/runtime/types"
	"github.com/your-org/go-backend-template/internal/api"
	"github.com/your-org/go-backend-template/internal/domain"
	http2 "github.com/your-org/go-backend-template/internal/http"
	"github.com/your-org/go-backend-template/internal/middleware"
	"github.com/your-org/go-backend-template/internal/validator"
	"go.uber.org/zap"
)

// maxBodyBytes caps every JSON body the API accepts. 1 MiB is generous
// for a CRUD service and prevents memory-exhaustion DoS from oversized
// payloads.
const maxBodyBytes int64 = 1 << 20

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	return http2.DecodeJSON(w, r, maxBodyBytes, dst)
}

func writeDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, http2.ErrBodyTooLarge) {
		http2.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}
	http2.RespondError(w, http.StatusBadRequest, "invalid request body")
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
}

// UserResponse represents a user response
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	IsActive  bool      `json:"is_active"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

// Handler holds auth dependencies
type Handler struct {
	auth   AuthService
	admin  ApprovedUserAdminService
	logger *zap.Logger
}

// NewHandler creates a new auth handler. The auth and admin surfaces are
// injected separately so callers can narrow what they pass in.
func NewHandler(auth AuthService, admin ApprovedUserAdminService, logger *zap.Logger) *Handler {
	return &Handler{
		auth:   auth,
		admin:  admin,
		logger: logger,
	}
}

// RegisterUser implements api.ServerInterface.
func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if err := validator.ValidateRegister(validator.RegisterRequest{
		Email:      string(req.Email),
		Password:   req.Password,
		ApprovedID: req.ApprovedId,
	}); err != nil {
		http2.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.auth.Register(r.Context(), string(req.Email), req.Password, req.ApprovedId)
	if err != nil {
		h.logger.Error("register failed", zap.Error(err))
		if errors.Is(err, ErrUserNotFound) {
			http2.RespondError(w, http.StatusNotFound, "approved user not found")
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			http2.RespondError(w, http.StatusBadRequest, "invalid approved_id format")
			return
		}
		if errors.Is(err, ErrUserAlreadyExists) {
			http2.RespondError(w, http.StatusConflict, "user already exists")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusCreated, AuthResponse{
		Token:     token,
		TokenType: "bearer",
	})
}

// LoginUser implements api.ServerInterface.
func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if err := validator.ValidateLogin(validator.LoginRequest{
		Email:    string(req.Email),
		Password: req.Password,
	}); err != nil {
		http2.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.auth.Login(r.Context(), string(req.Email), req.Password)
	if err != nil {
		h.logger.Error("login failed", zap.Error(err))
		if errors.Is(err, ErrInvalidCredentials) {
			http2.RespondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusOK, AuthResponse{
		Token:     token,
		TokenType: "bearer",
	})
}

// GetCurrentUser implements api.ServerInterface.
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roles := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roles[i] = role.Name
	}

	var email string
	var firstName string
	if user.ApprovedUser != nil {
		email = user.ApprovedUser.Email
		firstName = user.ApprovedUser.FirstName
	}

	http2.RespondJSON(w, http.StatusOK, UserResponse{
		ID:        user.ID.String(),
		Email:     email,
		FirstName: firstName,
		IsActive:  user.IsActive,
		Roles:     roles,
		CreatedAt: user.CreatedAt,
	})
}

// ApprovedUserResponse represents an approved user response
type ApprovedUserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toApprovedUserResponse(au *domain.ApprovedUser) *ApprovedUserResponse {
	if au == nil {
		return nil
	}
	return &ApprovedUserResponse{
		ID:        au.ID.String(),
		Email:     au.Email,
		FirstName: au.FirstName,
		CreatedAt: au.CreatedAt,
		UpdatedAt: au.UpdatedAt,
	}
}

func toApprovedUserResponses(aus []*domain.ApprovedUser) []*ApprovedUserResponse {
	result := make([]*ApprovedUserResponse, 0, len(aus))
	for _, au := range aus {
		result = append(result, toApprovedUserResponse(au))
	}
	return result
}

// ListApprovedUsers implements api.ServerInterface.
func (h *Handler) ListApprovedUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.admin.ListApprovedUsers(r.Context())
	if err != nil {
		h.logger.Error("list approved users failed", zap.Error(err))
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusOK, toApprovedUserResponses(users))
}

// CreateApprovedUser implements api.ServerInterface.
func (h *Handler) CreateApprovedUser(w http.ResponseWriter, r *http.Request) {
	var req api.ApprovedUserRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if err := validator.ValidateApprovedUser(validator.ApprovedUserRequest{
		Email:     string(req.Email),
		FirstName: req.FirstName,
	}); err != nil {
		http2.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get creator from context
	creator := middleware.UserFromContext(r.Context())
	if creator == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	approvedUser, err := h.admin.CreateApprovedUser(r.Context(), string(req.Email), req.FirstName, creator.ApprovedUserID)
	if err != nil {
		h.logger.Error("create approved user failed", zap.Error(err))
		if errors.Is(err, ErrApprovedEmailExists) {
			http2.RespondError(w, http.StatusConflict, "Email already in approved list")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusCreated, toApprovedUserResponse(approvedUser))
}

// BulkCreateApprovedUsers implements api.ServerInterface.
func (h *Handler) BulkCreateApprovedUsers(w http.ResponseWriter, r *http.Request) {
	var req api.BulkApprovedUserRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if len(req.Users) == 0 {
		http2.RespondError(w, http.StatusBadRequest, "users array is required")
		return
	}

	for _, u := range req.Users {
		if err := validator.ValidateApprovedUser(validator.ApprovedUserRequest{
			Email:     string(u.Email),
			FirstName: u.FirstName,
		}); err != nil {
			http2.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Get creator from context
	creator := middleware.UserFromContext(r.Context())
	if creator == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	emails := make([]string, len(req.Users))
	firstNames := make([]string, len(req.Users))
	for i, u := range req.Users {
		emails[i] = string(u.Email)
		firstNames[i] = u.FirstName
	}

	users, err := h.admin.BulkCreateApprovedUsers(r.Context(), emails, firstNames, creator.ApprovedUserID)
	if err != nil {
		h.logger.Error("bulk create approved users failed", zap.Error(err))
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusCreated, toApprovedUserResponses(users))
}

// DeleteApprovedUser implements api.ServerInterface.
func (h *Handler) DeleteApprovedUser(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	if err := h.admin.DeleteApprovedUser(r.Context(), id); err != nil {
		h.logger.Error("delete approved user failed", zap.Error(err))
		if errors.Is(err, ErrApprovedUserNotFound) {
			http2.RespondError(w, http.StatusNotFound, "approved user not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
