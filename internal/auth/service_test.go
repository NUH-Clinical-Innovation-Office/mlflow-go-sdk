package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// mockUserRepository is a hand-rolled in-memory test double for
// UserRepository. Only the methods exercised by Service tests carry
// state; the rest are nil/zero-valued and return safe defaults.
type mockUserRepository struct {
	getUserByEmail          func(ctx context.Context, email string) (*UserRow, error)
	getUserByID             func(ctx context.Context, id uuid.UUID) (*UserRow, error)
	approvedUserExists      func(ctx context.Context, id uuid.UUID) (bool, error)
	getApprovedByID         func(ctx context.Context, id uuid.UUID) (*ApprovedUserRow, error)
	getRolesByName          func(ctx context.Context, name string) (*RoleRow, error)
	assignRole              func(ctx context.Context, userID, roleID uuid.UUID) error
	createUser              func(ctx context.Context, in UserCreateInput) (*UserRow, error)
	getApprovedByEmail      func(ctx context.Context, email string) (*ApprovedUserRow, error)
	listApprovedUsers       func(ctx context.Context) ([]*ApprovedUserRow, error)
	createApprovedUser      func(ctx context.Context, in ApprovedUserCreateInput) (*ApprovedUserRow, error)
	bulkApproved            func(ctx context.Context, in BulkApprovedUserInput) ([]*ApprovedUserRow, error)
	deleteApproved          func(ctx context.Context, id uuid.UUID) error
	getWithRolesAndApproved func(ctx context.Context, id uuid.UUID) (*UserWithContext, error)
}

func (m *mockUserRepository) GetUserByEmail(ctx context.Context, email string) (*UserRow, error) {
	if m.getUserByEmail == nil {
		return nil, assert.AnError
	}
	return m.getUserByEmail(ctx, email)
}
func (m *mockUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*UserRow, error) {
	if m.getUserByID == nil {
		return nil, assert.AnError
	}
	return m.getUserByID(ctx, id)
}
func (m *mockUserRepository) CreateUser(ctx context.Context, in UserCreateInput) (*UserRow, error) {
	if m.createUser == nil {
		return nil, assert.AnError
	}
	return m.createUser(ctx, in)
}
func (m *mockUserRepository) GetApprovedUserByID(ctx context.Context, id uuid.UUID) (*ApprovedUserRow, error) {
	if m.getApprovedByID == nil {
		return nil, assert.AnError
	}
	return m.getApprovedByID(ctx, id)
}
func (m *mockUserRepository) ApprovedUserExists(ctx context.Context, id uuid.UUID) (bool, error) {
	if m.approvedUserExists == nil {
		return false, assert.AnError
	}
	return m.approvedUserExists(ctx, id)
}
func (m *mockUserRepository) GetUserRoles(_ context.Context, _ uuid.UUID) ([]RoleRow, error) {
	return nil, nil
}
func (m *mockUserRepository) GetRoleByName(ctx context.Context, name string) (*RoleRow, error) {
	if m.getRolesByName == nil {
		return nil, nil
	}
	return m.getRolesByName(ctx, name)
}
func (m *mockUserRepository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	if m.assignRole == nil {
		return nil
	}
	return m.assignRole(ctx, userID, roleID)
}
func (m *mockUserRepository) ListApprovedUsers(ctx context.Context) ([]*ApprovedUserRow, error) {
	if m.listApprovedUsers == nil {
		return nil, nil
	}
	return m.listApprovedUsers(ctx)
}
func (m *mockUserRepository) CreateApprovedUser(ctx context.Context, in ApprovedUserCreateInput) (*ApprovedUserRow, error) {
	if m.createApprovedUser == nil {
		return nil, nil
	}
	return m.createApprovedUser(ctx, in)
}
func (m *mockUserRepository) BulkCreateApprovedUsers(ctx context.Context, in BulkApprovedUserInput) ([]*ApprovedUserRow, error) {
	if m.bulkApproved == nil {
		return nil, nil
	}
	return m.bulkApproved(ctx, in)
}
func (m *mockUserRepository) DeleteApprovedUser(ctx context.Context, id uuid.UUID) error {
	if m.deleteApproved == nil {
		return nil
	}
	return m.deleteApproved(ctx, id)
}
func (m *mockUserRepository) GetApprovedUserByEmail(ctx context.Context, email string) (*ApprovedUserRow, error) {
	if m.getApprovedByEmail == nil {
		return nil, assert.AnError
	}
	return m.getApprovedByEmail(ctx, email)
}
func (m *mockUserRepository) GetUserWithRolesAndApproved(ctx context.Context, id uuid.UUID) (*UserWithContext, error) {
	if m.getWithRolesAndApproved == nil {
		return nil, assert.AnError
	}
	return m.getWithRolesAndApproved(ctx, id)
}

func newTestService(t *testing.T, repo UserRepository) *Service {
	t.Helper()
	return NewService(repo, "test-secret-key", time.Hour, 4)
}

func bcryptHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 4)
	require.NoError(t, err)
	return string(h)
}

// TestService_Login_JWT_IsStringUserID is a regression test for the
// Login-issued-token bug: the user_id JWT claim must be a string,
// otherwise GetUserFromToken's type assertion fails on every call.
func TestService_Login_JWT_IsStringUserID(t *testing.T) {
	userID := uuid.New()
	approvedID := uuid.New()
	hash := bcryptHash(t, "Password123")

	repo := &mockUserRepository{
		getUserByEmail: func(_ context.Context, _ string) (*UserRow, error) {
			return &UserRow{
				ID:             userID,
				ApprovedUserID: approvedID,
				Email:          "x@example.com",
				PasswordHash:   hash,
				IsActive:       true,
			}, nil
		},
	}
	svc := newTestService(t, repo)

	tokenStr, err := svc.Login(context.Background(), "x@example.com", "Password123")
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	parsed, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
	require.NoError(t, err)
	claims := parsed.Claims.(jwt.MapClaims)
	uid, ok := claims["user_id"].(string)
	require.True(t, ok, "user_id claim must be a string, got %T", claims["user_id"])
	assert.Equal(t, userID.String(), uid)
}

// TestService_Login_InactiveUserRejected covers the IsActive-in-Login bug:
// a deactivated user must not authenticate.
func TestService_Login_InactiveUserRejected(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmail: func(_ context.Context, _ string) (*UserRow, error) {
			return &UserRow{
				ID:             uuid.New(),
				ApprovedUserID: uuid.New(),
				Email:          "x@example.com",
				PasswordHash:   bcryptHash(t, "Password123"),
				IsActive:       false,
			}, nil
		},
	}
	svc := newTestService(t, repo)

	_, err := svc.Login(context.Background(), "x@example.com", "Password123")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestService_Login_BadPassword(t *testing.T) {
	repo := &mockUserRepository{
		getUserByEmail: func(_ context.Context, _ string) (*UserRow, error) {
			return &UserRow{
				ID:             uuid.New(),
				ApprovedUserID: uuid.New(),
				Email:          "x@example.com",
				PasswordHash:   bcryptHash(t, "Password123"),
				IsActive:       true,
			}, nil
		},
	}
	svc := newTestService(t, repo)

	_, err := svc.Login(context.Background(), "x@example.com", "WrongPassword")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

// TestService_GetUserFromToken_InactiveUserRejected ensures the token
// validation path also enforces IsActive (a deactivated user cannot
// keep using their previously-issued token).
func TestService_GetUserFromToken_InactiveUserRejected(t *testing.T) {
	userID := uuid.New()
	approvedID := uuid.New()

	secret := []byte("test-secret-key")
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"email":   "x@example.com",
		"exp":     time.Now().Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	require.NoError(t, err)

	repo := &mockUserRepository{
		getWithRolesAndApproved: func(_ context.Context, _ uuid.UUID) (*UserWithContext, error) {
			return &UserWithContext{
				User: &UserRow{
					ID:             userID,
					ApprovedUserID: approvedID,
					Email:          "x@example.com",
					IsActive:       false,
				},
				RoleNames: []string{},
			}, nil
		},
	}
	svc := NewService(repo, "test-secret-key", time.Hour, 4)

	_, err = svc.GetUserFromToken(context.Background(), signed)
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestService_Register_DuplicateUser(t *testing.T) {
	approvedID := uuid.New()
	repo := &mockUserRepository{
		approvedUserExists: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return true, nil
		},
		getUserByEmail: func(_ context.Context, _ string) (*UserRow, error) {
			return &UserRow{ID: uuid.New(), IsActive: true}, nil
		},
	}
	svc := newTestService(t, repo)

	_, err := svc.Register(context.Background(), "dup@example.com", "Password123", approvedID.String())
	assert.ErrorIs(t, err, ErrUserAlreadyExists)
}

func TestService_Register_ApprovedUserMissing(t *testing.T) {
	approvedID := uuid.New()
	repo := &mockUserRepository{
		approvedUserExists: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	svc := newTestService(t, repo)

	_, err := svc.Register(context.Background(), "new@example.com", "Password123", approvedID.String())
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestService_CreateApprovedUser_DuplicateEmail(t *testing.T) {
	repo := &mockUserRepository{
		getApprovedByEmail: func(_ context.Context, _ string) (*ApprovedUserRow, error) {
			return &ApprovedUserRow{ID: uuid.New()}, nil
		},
	}
	svc := newTestService(t, repo)

	_, err := svc.CreateApprovedUser(context.Background(), "dup@example.com", "First", uuid.New())
	assert.ErrorIs(t, err, ErrApprovedEmailExists)
}

// TestToDomainUser covers the pure mapper function directly. It has no
// dependencies, so a unit test is the right level.
func TestToDomainUser(t *testing.T) {
	user := &UserRow{
		ID:             uuid.New(),
		ApprovedUserID: uuid.New(),
		Email:          "u@example.com",
		PasswordHash:   "hash",
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	approved := &ApprovedUserRow{
		ID:        uuid.New(),
		Email:     "u@example.com",
		FirstName: "U",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	roles := []RoleRow{
		{ID: uuid.New(), Name: "user", CreatedAt: time.Now()},
		{ID: uuid.New(), Name: "admin", CreatedAt: time.Now()},
	}

	d := ToDomainUser(user, approved, roles)
	require.NotNil(t, d)
	assert.Equal(t, user.ID, d.ID)
	assert.NotNil(t, d.ApprovedUser)
	assert.Equal(t, approved.Email, d.ApprovedUser.Email)
	assert.Len(t, d.Roles, 2)
	assert.True(t, d.HasRole("admin"))
}
