// Package auth provides authentication service.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/your-org/go-backend-template/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// JWT claim keys used in tokens issued by this service. Centralized as
// constants so the issuer and the validator never drift.
const (
	claimUserID = "user_id"
	claimEmail  = "email"
	claimExp    = "exp"
)

// Service provides authentication business logic
type Service struct {
	repo       UserRepository
	jwtSecret  []byte
	jwtExpiry  time.Duration
	bcryptCost int
}

// NewService creates a new auth service
func NewService(repo UserRepository, jwtSecret string, jwtExpiry time.Duration, bcryptCost int) *Service {
	return &Service{
		repo:       repo,
		jwtSecret:  []byte(jwtSecret),
		jwtExpiry:  jwtExpiry,
		bcryptCost: bcryptCost,
	}
}

// Register registers a new user
func (s *Service) Register(ctx context.Context, email, password, approvedID string) (string, error) {
	approvedUUID, err := uuid.Parse(approvedID)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	exists, err := s.repo.ApprovedUserExists(ctx, approvedUUID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrUserNotFound
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil && existing != nil {
		return "", ErrUserAlreadyExists
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", err
	}

	user, err := s.repo.CreateUser(ctx, UserCreateInput{
		ApprovedUserID: approvedUUID,
		Email:          email,
		PasswordHash:   string(hashedPassword),
		IsActive:       true,
	})
	if err != nil {
		return "", err
	}

	if assignErr := s.assignDefaultRole(ctx, user.ID); assignErr != nil {
		return "", assignErr
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		claimUserID: user.ID.String(),
		claimEmail:  user.Email,
		claimExp:    time.Now().Add(s.jwtExpiry).Unix(),
	})

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// assignDefaultRole assigns the default "user" role from the
// process-startup cache. If the cache is empty (e.g. dev without
// migrations), it looks the role up once and populates the cache.
func (s *Service) assignDefaultRole(ctx context.Context, userID uuid.UUID) error {
	roleID := DefaultUserRoleID()
	if roleID == uuid.Nil {
		userRole, err := s.repo.GetRoleByName(ctx, "user")
		if err == nil && userRole != nil {
			roleID = userRole.ID
			SetDefaultUserRoleID(roleID)
		}
	}
	if roleID == uuid.Nil {
		return nil
	}
	return s.repo.AssignRoleToUser(ctx, userID, roleID)
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	if !user.IsActive {
		return "", ErrInvalidCredentials
	}

	if cmpErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); cmpErr != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		claimUserID: user.ID.String(),
		claimEmail:  user.Email,
		claimExp:    time.Now().Add(s.jwtExpiry).Unix(),
	})

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GetUserFromToken validates a JWT token and returns the user. The
// user, role names, and approved_user link are fetched in a single
// roundtrip; any DB error is propagated (fail-closed) so transient
// failures do not silently downgrade the user to "no roles".
func (s *Service) GetUserFromToken(ctx context.Context, tokenString string) (*domain.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidCredentials
		}
		return s.jwtSecret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidCredentials
	}

	userID, ok := claims[claimUserID].(string)
	if !ok {
		return nil, ErrInvalidCredentials
	}

	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	agg, err := s.repo.GetUserWithRolesAndApproved(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if !agg.User.IsActive {
		return nil, ErrInvalidCredentials
	}

	// Translate the aggregate into the cross-feature domain type.
	return userToDomain(agg.User, agg.ApprovedUser, agg.RoleNames), nil
}

// userToDomain assembles a domain.User from the aggregate result. Inlined
// here so callers do not need a separate mapper file for one call site.
func userToDomain(user *UserRow, approved *ApprovedUserRow, roleNames []string) *domain.User {
	domainRoles := make([]domain.Role, 0, len(roleNames))
	for _, name := range roleNames {
		domainRoles = append(domainRoles, domain.Role{Name: name})
	}
	var approvedDomain *domain.ApprovedUser
	if approved != nil {
		approvedDomain = &domain.ApprovedUser{
			ID:        approved.ID,
			Email:     approved.Email,
			FirstName: approved.FirstName,
			CreatedBy: approved.CreatedBy,
			CreatedAt: approved.CreatedAt,
			UpdatedAt: approved.UpdatedAt,
		}
	}
	return &domain.User{
		ID:             user.ID,
		ApprovedUserID: user.ApprovedUserID,
		HashedPassword: user.PasswordHash,
		IsActive:       user.IsActive,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		Roles:          domainRoles,
		ApprovedUser:   approvedDomain,
	}
}

// ListApprovedUsers lists all approved users (admin only)
func (s *Service) ListApprovedUsers(ctx context.Context) ([]*domain.ApprovedUser, error) {
	rows, err := s.repo.ListApprovedUsers(ctx)
	if err != nil {
		return nil, err
	}
	return approvedUsersToDomain(rows), nil
}

// CreateApprovedUser creates a new approved user (admin only)
func (s *Service) CreateApprovedUser(ctx context.Context, email, firstName string, createdBy uuid.UUID) (*domain.ApprovedUser, error) {
	if existing, err := s.repo.GetApprovedUserByEmail(ctx, email); err == nil && existing != nil {
		return nil, ErrApprovedEmailExists
	}

	row, err := s.repo.CreateApprovedUser(ctx, ApprovedUserCreateInput{
		Email:     email,
		FirstName: firstName,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, err
	}
	return approvedUserToDomain(row), nil
}

// BulkCreateApprovedUsers creates multiple approved users (admin only)
func (s *Service) BulkCreateApprovedUsers(ctx context.Context, emails, firstNames []string, createdBy uuid.UUID) ([]*domain.ApprovedUser, error) {
	if len(emails) != len(firstNames) {
		return nil, ErrInvalidInput
	}
	rows, err := s.repo.BulkCreateApprovedUsers(ctx, BulkApprovedUserInput{
		Emails:     emails,
		FirstNames: firstNames,
		CreatedBy:  createdBy,
	})
	if err != nil {
		return nil, err
	}
	return approvedUsersToDomain(rows), nil
}

// DeleteApprovedUser deletes an approved user (admin only)
func (s *Service) DeleteApprovedUser(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteApprovedUser(ctx, id)
}

// approvedUserToDomain wraps a row into the cross-feature domain type.
func approvedUserToDomain(r *ApprovedUserRow) *domain.ApprovedUser {
	if r == nil {
		return nil
	}
	return &domain.ApprovedUser{
		ID:        r.ID,
		Email:     r.Email,
		FirstName: r.FirstName,
		CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func approvedUsersToDomain(rs []*ApprovedUserRow) []*domain.ApprovedUser {
	out := make([]*domain.ApprovedUser, len(rs))
	for i, r := range rs {
		out[i] = approvedUserToDomain(r)
	}
	return out
}
