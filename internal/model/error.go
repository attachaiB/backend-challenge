package model

import "errors"

// Errors returned by the service layer. The controller maps them to HTTP status codes.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email is already registered")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type ErrorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}
