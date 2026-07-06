// Package auth provides authentication repository.
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	dbutil "github.com/your-org/go-backend-template/internal/db/dbutil"
	db "github.com/your-org/go-backend-template/internal/db/sqlc"
)

// Repository provides database access for auth. All methods return
// feature-local row DTOs (UserRow, ApprovedUserRow, RoleRow) so the
// service layer never imports internal/db/sqlc or pgtype.
type Repository struct {
	db *db.Queries
}

// NewRepository creates a new auth repository.
func NewRepository(q *db.Queries) *Repository {
	return &Repository{db: q}
}

// GetUserByEmail gets a user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*UserRow, error) {
	u, err := r.db.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return userRowFromDB(&u), nil
}

// GetUserByID gets a user by ID.
func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*UserRow, error) {
	u, err := r.db.GetUserByID(ctx, dbutil.UUIDToPgtypeValue(id))
	if err != nil {
		return nil, err
	}
	return userRowFromDB(&u), nil
}

// CreateUser creates a new user.
func (r *Repository) CreateUser(ctx context.Context, in UserCreateInput) (*UserRow, error) {
	u, err := r.db.CreateUser(ctx, in.toCreateParams())
	if err != nil {
		return nil, err
	}
	return userRowFromDB(&u), nil
}

// GetApprovedUserByID gets an approved user by ID.
func (r *Repository) GetApprovedUserByID(ctx context.Context, id uuid.UUID) (*ApprovedUserRow, error) {
	a, err := r.db.GetApprovedUserByID(ctx, dbutil.UUIDToPgtypeValue(id))
	if err != nil {
		return nil, err
	}
	return approvedUserRowFromDB(&a), nil
}

// GetUserRoles gets roles for a user.
func (r *Repository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]RoleRow, error) {
	rows, err := r.db.GetUserRoles(ctx, dbutil.UUIDToPgtypeValue(userID))
	if err != nil {
		return nil, err
	}
	return rolesFromDB(rows), nil
}

// GetRoleByName gets a role by name. Returns (nil, nil) when the role
// does not exist; other errors are returned as-is.
func (r *Repository) GetRoleByName(ctx context.Context, name string) (*RoleRow, error) {
	row, err := r.db.GetRoleByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r0 := roleRowFromDB(&row)
	return &r0, nil
}

// ApprovedUserExists reports whether a row with the given id is
// present. The implementation uses an EXISTS subquery so the lookup
// stops at the first match.
func (r *Repository) ApprovedUserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return r.db.ApprovedUserExists(ctx, dbutil.UUIDToPgtypeValue(id))
}

// AssignRoleToUser assigns a role to a user.
func (r *Repository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.db.AssignRole(ctx, db.AssignRoleParams{
		UserID: dbutil.UUIDToPgtypeValue(userID),
		RoleID: dbutil.UUIDToPgtypeValue(roleID),
	})
}

// ListApprovedUsers lists all approved users.
func (r *Repository) ListApprovedUsers(ctx context.Context) ([]*ApprovedUserRow, error) {
	rows, err := r.db.ListApprovedUsers(ctx)
	if err != nil {
		return nil, err
	}
	return approvedUsersFromDB(rows), nil
}

// CreateApprovedUser creates a new approved user.
func (r *Repository) CreateApprovedUser(ctx context.Context, in ApprovedUserCreateInput) (*ApprovedUserRow, error) {
	row, err := r.db.CreateApprovedUser(ctx, in.toCreateParams())
	if err != nil {
		return nil, err
	}
	return approvedUserRowFromDB(&row), nil
}

// BulkCreateApprovedUsers creates multiple approved users.
func (r *Repository) BulkCreateApprovedUsers(ctx context.Context, in BulkApprovedUserInput) ([]*ApprovedUserRow, error) {
	if len(in.Emails) != len(in.FirstNames) {
		return nil, ErrInvalidInput
	}
	rows, err := r.db.CreateApprovedUsersBulk(ctx, in.toCreateParams())
	if err != nil {
		return nil, err
	}
	return approvedUsersFromDB(rows), nil
}

// DeleteApprovedUser deletes an approved user. Returns
// ErrApprovedUserNotFound when no row matched the id, so callers can
// surface 404 instead of silently succeeding.
func (r *Repository) DeleteApprovedUser(ctx context.Context, id uuid.UUID) error {
	rows, err := r.db.DeleteApprovedUser(ctx, dbutil.UUIDToPgtypeValue(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrApprovedUserNotFound
	}
	return nil
}

// UserWithContext is the result of the single-query aggregate. The
// caller (service) translates this into a domain.User.
type UserWithContext struct {
	User         *UserRow
	RoleNames    []string
	ApprovedUser *ApprovedUserRow // nil when the user has no approved_user link
}

// GetUserWithRolesAndApproved returns the user, their role names, and
// their approved_user link in one roundtrip. Used by the auth
// middleware on every request.
func (r *Repository) GetUserWithRolesAndApproved(ctx context.Context, id uuid.UUID) (*UserWithContext, error) {
	row, err := r.db.GetUserWithRolesAndApproved(ctx, dbutil.UUIDToPgtypeValue(id))
	if err != nil {
		return nil, err
	}
	user := userRowFromAggregate(&row)
	roles := roleNamesFromAggregate(row.RoleNames)
	return &UserWithContext{
		User:         user,
		RoleNames:    roles,
		ApprovedUser: approvedUserRowFromAggregate(&row),
	}, nil
}

// GetApprovedUserByEmail gets an approved user by email.
func (r *Repository) GetApprovedUserByEmail(ctx context.Context, email string) (*ApprovedUserRow, error) {
	a, err := r.db.GetApprovedUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return approvedUserRowFromDB(&a), nil
}
