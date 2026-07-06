// Package dbutil provides shared conversion helpers.
package dbutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// PgUUIDToPtr converts a pgtype.UUID to *uuid.UUID
func PgUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	u := uuid.UUID(p.Bytes)
	return &u
}

// UUIDToPgtype converts *uuid.UUID to pgtype.UUID
func UUIDToPgtype(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// UUIDToPgtypeValue converts a uuid.UUID (by value) to pgtype.UUID. The
// returned value is always Valid; callers that may want to represent
// a "null" UUID should use *uuid.UUID + UUIDToPgtype instead.
func UUIDToPgtypeValue(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// PgUUIDToValue converts a pgtype.UUID to uuid.UUID. Invalid input
// yields uuid.Nil; callers that need to distinguish nil from zero
// should use PgUUIDToPtr instead.
func PgUUIDToValue(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// PgTimestamptzToPtr converts pgtype.Timestamptz to *time.Time
func PgTimestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// PgInt4ToPtr converts pgtype.Int4 to *int
func PgInt4ToPtr(p pgtype.Int4) *int {
	if !p.Valid {
		return nil
	}
	v := int(p.Int32)
	return &v
}

// IntToPgInt4 converts *int to pgtype.Int4
func IntToPgInt4(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}
