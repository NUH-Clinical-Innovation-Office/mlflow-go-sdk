// Package auth provides authentication and account management.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// UserRow is the persistence-shape view of a row in the users table.
// The service and mapper operate on this struct; they do not import
// internal/db/sqlc or pgtype.
type UserRow struct {
	ID             uuid.UUID
	ApprovedUserID uuid.UUID
	Email          string
	PasswordHash   string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ApprovedUserRow is the persistence-shape view of a row in the
// approved_users table. CreatedBy is a pointer because the column
// itself is nullable (FK with ON DELETE SET NULL).
type ApprovedUserRow struct {
	ID        uuid.UUID
	Email     string
	FirstName string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RoleRow is the persistence-shape view of a row in the roles table.
type RoleRow struct {
	ID          uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
}

// UserCreateInput is what the service hands to the repository to
// create a new user. It carries only the columns the service is
// allowed to set; the repository fills timestamps.
type UserCreateInput struct {
	ApprovedUserID uuid.UUID
	Email          string
	PasswordHash   string
	IsActive       bool
}

// ApprovedUserCreateInput is what the service hands to the repository
// to create a new approved_user row.
type ApprovedUserCreateInput struct {
	Email     string
	FirstName string
	CreatedBy uuid.UUID
}

// BulkApprovedUserInput is what the service hands to the repository
// to bulk-create approved_user rows. Emails and FirstNames must be
// the same length; the repository enforces that.
type BulkApprovedUserInput struct {
	Emails     []string
	FirstNames []string
	CreatedBy  uuid.UUID
}
