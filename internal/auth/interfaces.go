// Package auth provides authentication interfaces for dependency injection.
package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/your-org/go-backend-template/internal/domain"
)

// UserRepository defines the interface for user data access. Methods
// return feature-local row DTOs; the interface has no dependency on
// internal/db/sqlc.
type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*UserRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*UserRow, error)
	CreateUser(ctx context.Context, in UserCreateInput) (*UserRow, error)
	ApprovedUserExists(ctx context.Context, id uuid.UUID) (bool, error)
	GetApprovedUserByID(ctx context.Context, id uuid.UUID) (*ApprovedUserRow, error)
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]RoleRow, error)
	GetRoleByName(ctx context.Context, name string) (*RoleRow, error)
	AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error
	ListApprovedUsers(ctx context.Context) ([]*ApprovedUserRow, error)
	CreateApprovedUser(ctx context.Context, in ApprovedUserCreateInput) (*ApprovedUserRow, error)
	BulkCreateApprovedUsers(ctx context.Context, in BulkApprovedUserInput) ([]*ApprovedUserRow, error)
	DeleteApprovedUser(ctx context.Context, id uuid.UUID) error
	GetApprovedUserByEmail(ctx context.Context, email string) (*ApprovedUserRow, error)
	GetUserWithRolesAndApproved(ctx context.Context, id uuid.UUID) (*UserWithContext, error)
}

// AuthService is the authentication-focused surface. Middleware that only
// needs to validate tokens depends on this and nothing else.
type AuthService interface {
	Register(ctx context.Context, email, password, approvedID string) (string, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetUserFromToken(ctx context.Context, token string) (*domain.User, error)
}

// ApprovedUserAdminService is the admin surface for managing the approved
// users whitelist. Kept separate from AuthService so handlers and the
// dependency graph don't pull in admin operations they don't need.
type ApprovedUserAdminService interface {
	ListApprovedUsers(ctx context.Context) ([]*domain.ApprovedUser, error)
	CreateApprovedUser(ctx context.Context, email, firstName string, createdBy uuid.UUID) (*domain.ApprovedUser, error)
	BulkCreateApprovedUsers(ctx context.Context, emails, firstNames []string, createdBy uuid.UUID) ([]*domain.ApprovedUser, error)
	DeleteApprovedUser(ctx context.Context, id uuid.UUID) error
}
