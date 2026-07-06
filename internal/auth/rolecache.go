package auth

import (
	"context"
	"sync/atomic"

	"github.com/google/uuid"
)

// defaultUserRoleID caches the UUID of the seeded "user" role so Register
// does not have to query it on every call. The cache is process-local;
// each replica of the API computes it once at startup.
//
// Storing as an atomic.Value so reads are lock-free.
var defaultUserRoleID atomic.Value // uuid.UUID or nil

// InitDefaultRoles resolves the "user" role at startup and stores its
// ID. Safe to call multiple times; only the first call hits the DB.
func InitDefaultRoles(ctx context.Context, repo UserRepository) error {
	if v := defaultUserRoleID.Load(); v != nil {
		return nil
	}
	r, err := repo.GetRoleByName(ctx, "user")
	if err != nil {
		return err
	}
	if r == nil {
		return ErrUserNotFound
	}
	defaultUserRoleID.Store(r.ID)
	return nil
}

// DefaultUserRoleID returns the cached "user" role ID, or uuid.Nil if
// the cache has not been populated. Register handles the missing case
// by performing a one-shot lookup and re-priming the cache.
func DefaultUserRoleID() uuid.UUID {
	if v := defaultUserRoleID.Load(); v != nil {
		if id, ok := v.(uuid.UUID); ok {
			return id
		}
	}
	return uuid.Nil
}

// SetDefaultUserRoleID primes the cache (used by the lazy fallback in
// Register when InitDefaultRoles has not been called).
func SetDefaultUserRoleID(id uuid.UUID) {
	defaultUserRoleID.Store(id)
}
