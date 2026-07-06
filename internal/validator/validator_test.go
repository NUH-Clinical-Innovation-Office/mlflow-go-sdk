package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{"valid email", "test@example.com", nil},
		{"valid email with subdomain", "test.user@mail.example.co.uk", nil},
		{"valid email with plus", "test+tag@example.com", nil},
		{"empty email", "", ErrEmailRequired},
		{"whitespace only", "   ", ErrEmailRequired},
		{"missing @", "testexample.com", ErrEmailInvalid},
		{"missing domain", "test@", ErrEmailInvalid},
		{"missing local", "@example.com", ErrEmailInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid password", "Password1", nil},
		{"valid long password", "MySecurePassword123!", nil},
		{"empty password", "", ErrPasswordRequired},
		{"too short", "Pass1", ErrPasswordTooShort},
		{"exactly 7 chars", "Passw0r", ErrPasswordTooShort},
		{"no uppercase", "password1", ErrPasswordNoUpper},
		{"no lowercase", "PASSWORD1", ErrPasswordNoLower},
		{"no digit", "Password!", ErrPasswordNoDigit},
		{"all lowercase with digit", "password1", ErrPasswordNoUpper},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr error
	}{
		{"valid title", "My Todo", nil},
		{"valid title with spaces", "  My Todo  ", nil},
		{"valid single char", "A", nil},
		{"empty title", "", ErrTitleRequired},
		{"whitespace only", "   ", ErrTitleRequired},
		{"too long", string(make([]byte, 501)), ErrTitleTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTitle(tt.title)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFirstName(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		wantErr   error
	}{
		{"valid name", "John", nil},
		{"valid name with space", "John Paul", nil},
		{"valid name with hyphen", "Mary-Jane", nil},
		{"valid name with apostrophe", "O'Brien", nil},
		{"empty name", "", ErrFirstNameRequired},
		{"whitespace only", "   ", ErrFirstNameRequired},
		{"name with numbers", "John1", ErrFirstNameInvalid},
		{"name with special chars", "John!", ErrFirstNameInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFirstName(tt.firstName)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestRegisterRequestValidator(t *testing.T) {
	tests := []struct {
		name    string
		req     RegisterRequest
		wantErr bool
	}{
		{"valid request", RegisterRequest{Email: "test@example.com", Password: "Password1", ApprovedID: "uuid"}, false},
		{"invalid email", RegisterRequest{Email: "invalid", Password: "Password1", ApprovedID: "uuid"}, true},
		{"invalid password", RegisterRequest{Email: "test@example.com", Password: "weak", ApprovedID: "uuid"}, true},
		{"missing approved_id", RegisterRequest{Email: "test@example.com", Password: "Password1", ApprovedID: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegister(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoginRequestValidator(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		{"valid request", LoginRequest{Email: "test@example.com", Password: "Password1"}, false},
		{"invalid email", LoginRequest{Email: "invalid", Password: "Password1"}, true},
		{"missing password", LoginRequest{Email: "test@example.com", Password: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLogin(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateTodoRequestValidator(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"valid request", "My Todo", false},
		{"empty title", "", true},
		{"whitespace title", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreateTodoTitle(tt.title)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApprovedUserRequestValidator(t *testing.T) {
	tests := []struct {
		name    string
		req     ApprovedUserRequest
		wantErr bool
	}{
		{"valid request", ApprovedUserRequest{Email: "test@example.com", FirstName: "John"}, false},
		{"invalid email", ApprovedUserRequest{Email: "invalid", FirstName: "John"}, true},
		{"invalid name", ApprovedUserRequest{Email: "test@example.com", FirstName: "John1"}, true},
		{"empty email", ApprovedUserRequest{Email: "", FirstName: "John"}, true},
		{"empty name", ApprovedUserRequest{Email: "test@example.com", FirstName: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateApprovedUser(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
