// Package validator provides request validation decorators.
package validator

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
)

var (
	ErrEmailRequired     = errors.New("email is required")
	ErrEmailInvalid      = errors.New("invalid email format")
	ErrPasswordRequired  = errors.New("password is required")
	ErrPasswordTooShort  = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong   = errors.New("password must be at most 72 characters")
	ErrPasswordNoUpper   = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower   = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoDigit   = errors.New("password must contain at least one digit")
	ErrTitleRequired     = errors.New("title is required")
	ErrTitleTooLong      = errors.New("title must be less than 500 characters")
	ErrFirstNameRequired = errors.New("first name is required")
	ErrFirstNameInvalid  = errors.New("first name contains invalid characters")
)

// Validator interface and the old struct-based validators are gone.
// Free functions (ValidateRegister, ValidateLogin, etc.) replace them.

// ValidateEmail checks email format. Uses net/mail.ParseAddress which
// handles the RFC 5322 grammar; no second regex check is required.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrEmailRequired
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return ErrEmailInvalid
	}
	return nil
}

// ValidatePassword checks password strength. The upper bound is 72 bytes
// which is bcrypt's silent-truncation ceiling; we reject longer input
// so the operator can return 400 instead of hashing a different value.
func ValidatePassword(password string) error {
	if password == "" {
		return ErrPasswordRequired
	}
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return ErrPasswordNoUpper
	}
	if !hasLower {
		return ErrPasswordNoLower
	}
	if !hasDigit {
		return ErrPasswordNoDigit
	}

	return nil
}

// ValidateTitle checks title validity
func ValidateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrTitleRequired
	}
	if len(title) > 500 {
		return ErrTitleTooLong
	}
	return nil
}

// ValidateFirstName checks first name validity
func ValidateFirstName(firstName string) error {
	firstName = strings.TrimSpace(firstName)
	if firstName == "" {
		return ErrFirstNameRequired
	}
	// Allow letters, spaces, hyphens, and apostrophes
	for _, r := range firstName {
		if !unicode.IsLetter(r) && r != ' ' && r != '-' && r != '\'' {
			return ErrFirstNameInvalid
		}
	}
	return nil
}

// --- Request types (live next to their validator to keep changes local) ---

// RegisterRequest is the create-account payload.
type RegisterRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	ApprovedID string `json:"approved_id"`
}

// ValidateRegister validates a RegisterRequest.
func ValidateRegister(r RegisterRequest) error {
	if err := ValidateEmail(r.Email); err != nil {
		return err
	}
	if err := ValidatePassword(r.Password); err != nil {
		return err
	}
	if r.ApprovedID == "" {
		return errors.New("approved_id is required")
	}
	return nil
}

// LoginRequest is the credential payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ValidateLogin validates a LoginRequest.
func ValidateLogin(r LoginRequest) error {
	if err := ValidateEmail(r.Email); err != nil {
		return err
	}
	if r.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}

// ValidateCreateTodoTitle validates the title field of a create-todo
// payload. Handlers call this with req.Title directly to avoid carrying
// a duplicate request type in both handler and validator packages.
func ValidateCreateTodoTitle(title string) error {
	return ValidateTitle(title)
}

// ValidateUpdateTodoTitle validates the optional title field of a
// PATCH-todo payload. nil pointer is rejected (every PATCH must touch
// title); non-nil pointer is checked for emptiness/length.
func ValidateUpdateTodoTitle(title *string) error {
	if title == nil {
		return ErrTitleRequired
	}
	return ValidateTitle(*title)
}

// ApprovedUserRequest is the create-approved-user payload.
type ApprovedUserRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
}

// ValidateApprovedUser validates an ApprovedUserRequest.
func ValidateApprovedUser(r ApprovedUserRequest) error {
	if err := ValidateEmail(r.Email); err != nil {
		return err
	}
	if err := ValidateFirstName(r.FirstName); err != nil {
		return err
	}
	return nil
}
