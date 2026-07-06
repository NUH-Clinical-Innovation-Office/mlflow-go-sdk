package auth

import (
	"github.com/jackc/pgx/v5/pgtype"
	dbutil "github.com/your-org/go-backend-template/internal/db/dbutil"
	db "github.com/your-org/go-backend-template/internal/db/sqlc"
)

// userRowFromDB converts a sqlc-generated user row into the feature
// row DTO. Returns nil for nil input.
func userRowFromDB(u *db.User) *UserRow {
	if u == nil {
		return nil
	}
	return &UserRow{
		ID:             dbutil.PgUUIDToValue(u.ID),
		ApprovedUserID: dbutil.PgUUIDToValue(u.ApprovedUserID),
		Email:          u.Email,
		PasswordHash:   u.PasswordHash,
		IsActive:       u.IsActive,
		CreatedAt:      u.CreatedAt.Time,
		UpdatedAt:      u.UpdatedAt.Time,
	}
}

// toCreateParams builds the sqlc param struct for CreateUser from a
// service-supplied input. Only the repository uses this; it is the
// last place in the auth package that touches db.CreateUserParams.
func (in UserCreateInput) toCreateParams() db.CreateUserParams {
	return db.CreateUserParams{
		ApprovedUserID: dbutil.UUIDToPgtypeValue(in.ApprovedUserID),
		Email:          in.Email,
		PasswordHash:   in.PasswordHash,
		IsActive:       in.IsActive,
	}
}

// approvedUserRowFromDB converts a sqlc-generated approved_user row.
func approvedUserRowFromDB(a *db.ApprovedUser) *ApprovedUserRow {
	if a == nil {
		return nil
	}
	return &ApprovedUserRow{
		ID:        dbutil.PgUUIDToValue(a.ID),
		Email:     a.Email,
		FirstName: a.FirstName,
		CreatedBy: dbutil.PgUUIDToPtr(a.CreatedBy),
		CreatedAt: a.CreatedAt.Time,
		UpdatedAt: a.UpdatedAt.Time,
	}
}

// toCreateParams builds the sqlc param struct for CreateApprovedUser.
func (in ApprovedUserCreateInput) toCreateParams() db.CreateApprovedUserParams {
	return db.CreateApprovedUserParams{
		Email:     in.Email,
		FirstName: in.FirstName,
		CreatedBy: dbutil.UUIDToPgtypeValue(in.CreatedBy),
	}
}

// roleRowFromDB converts a sqlc-generated role row.
func roleRowFromDB(r *db.Role) RoleRow {
	return RoleRow{
		ID:          dbutil.PgUUIDToValue(r.ID),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt.Time,
	}
}

// rolesFromDB converts a slice of sqlc role rows in one pass. nil
// input yields a non-nil zero-length slice so callers can range over
// the result without a nil check.
func rolesFromDB(rs []db.Role) []RoleRow {
	out := make([]RoleRow, len(rs))
	for i := range rs {
		out[i] = roleRowFromDB(&rs[i])
	}
	return out
}

// approvedUsersFromDB converts a slice of sqlc approved_user rows.
func approvedUsersFromDB(rs []db.ApprovedUser) []*ApprovedUserRow {
	out := make([]*ApprovedUserRow, len(rs))
	for i := range rs {
		out[i] = approvedUserRowFromDB(&rs[i])
	}
	return out
}

// bulkCreateParams builds the sqlc param struct for bulk approved_user
// inserts. The three input slices must all be the same length.
func (in BulkApprovedUserInput) toCreateParams() db.CreateApprovedUsersBulkParams {
	createdBy := make([]pgtype.UUID, len(in.Emails))
	createdByVal := dbutil.UUIDToPgtypeValue(in.CreatedBy)
	for i := range createdBy {
		createdBy[i] = createdByVal
	}
	return db.CreateApprovedUsersBulkParams{
		Column1: append([]string(nil), in.Emails...),
		Column2: append([]string(nil), in.FirstNames...),
		Column3: createdBy,
	}
}

// userRowFromAggregate converts the single-query aggregate result into
// a UserRow. Always non-nil for a non-nil aggregate row.
func userRowFromAggregate(r *db.GetUserWithRolesAndApprovedRow) *UserRow {
	if r == nil {
		return nil
	}
	return &UserRow{
		ID:             dbutil.PgUUIDToValue(r.ID),
		ApprovedUserID: dbutil.PgUUIDToValue(r.ApprovedUserID),
		Email:          r.Email,
		PasswordHash:   r.PasswordHash,
		IsActive:       r.IsActive,
		CreatedAt:      r.CreatedAt.Time,
		UpdatedAt:      r.UpdatedAt.Time,
	}
}

// approvedUserRowFromAggregate extracts the LEFT JOINed approved_user
// from the aggregate row. Returns nil when no approved_user is linked
// (the join's nullable columns are all zero values).
func approvedUserRowFromAggregate(r *db.GetUserWithRolesAndApprovedRow) *ApprovedUserRow {
	if r == nil || !r.ApprovedID.Valid {
		return nil
	}
	return &ApprovedUserRow{
		ID:        dbutil.PgUUIDToValue(r.ApprovedID),
		Email:     derefStr(r.ApprovedEmail),
		FirstName: derefStr(r.ApprovedFirstName),
		CreatedBy: dbutil.PgUUIDToPtr(r.ApprovedCreatedBy),
		CreatedAt: r.ApprovedCreatedAt.Time,
		UpdatedAt: r.ApprovedUpdatedAt.Time,
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// roleNamesFromAggregate converts the array_agg result (decoded by pgx
// as []any since the destination type is any) into
// []string.
func roleNamesFromAggregate(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	roles := make([]string, 0, len(raw))
	for _, r := range raw {
		if name, ok := r.(string); ok {
			roles = append(roles, name)
		}
	}
	return roles
}
