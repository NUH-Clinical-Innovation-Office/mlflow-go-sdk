package auth

import "github.com/your-org/go-backend-template/internal/domain"

// ToDomainUser assembles a rich domain.User from the row DTOs. The
// approved user and roles slice may be nil; the function never panics
// on nil input and returns a usable *domain.User either way.
func ToDomainUser(user *UserRow, approved *ApprovedUserRow, roles []RoleRow) *domain.User {
	domainRoles := make([]domain.Role, len(roles))
	for i, r := range roles {
		domainRoles[i] = domain.Role{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
		}
	}

	var approvedUserDomain *domain.ApprovedUser
	if approved != nil {
		approvedUserDomain = &domain.ApprovedUser{
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
		ApprovedUser:   approvedUserDomain,
	}
}
