// Package auth provides authentication and account management.
package auth

import "errors"

// Sentinel errors returned by the auth service. Use errors.Is to test.
var (
	// ErrInvalidCredentials covers bad email, bad password, inactive user,
	// and any other "this caller may not proceed" condition. Mapping them
	// all to one sentinel prevents the API from leaking which condition
	// triggered the failure.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUserNotFound is returned when an explicit user lookup fails
	// (e.g. registration with a non-existent approved_id).
	ErrUserNotFound = errors.New("user not found")

	// ErrUserAlreadyExists is returned when registering an email that is
	// already in the users table.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrApprovedEmailExists is returned by CreateApprovedUser when the
	// email is already in the approved_users table.
	ErrApprovedEmailExists = errors.New("email already in approved list")

	// ErrApprovedUserNotFound is returned by DeleteApprovedUser when the
	// target row does not exist.
	ErrApprovedUserNotFound = errors.New("approved user not found")

	// ErrInvalidInput is returned by the repository when its caller
	// passes inconsistent input (e.g. emails and first_names slices of
	// different lengths in a bulk approved-user insert).
	ErrInvalidInput = errors.New("invalid input")
)
