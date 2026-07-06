// Package domain provides shared domain models.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID             uuid.UUID
	ApprovedUserID uuid.UUID
	HashedPassword string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Eager loaded
	ApprovedUser *ApprovedUser
	Roles        []Role
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(roleName string) bool {
	for _, role := range u.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// ApprovedUser represents an approved user who can create accounts
type ApprovedUser struct {
	ID        uuid.UUID
	Email     string
	FirstName string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Role represents a user role
type Role struct {
	ID          uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
}

// Todo represents a todo item
type Todo struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Title       string
	Description *string
	IsCompleted bool
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
