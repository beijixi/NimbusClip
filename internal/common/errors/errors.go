package errors

import "fmt"

// Code represents a machine-readable error code.
type Code string

const (
	// CodeInternal indicates an unexpected internal failure.
	CodeInternal Code = "internal"
	// CodeNotFound indicates that a requested resource was not found.
	CodeNotFound Code = "not_found"
	// CodeInvalidArgument indicates the client provided invalid data.
	CodeInvalidArgument Code = "invalid_argument"
	// CodeUnauthorized indicates that authentication failed.
	CodeUnauthorized Code = "unauthorized"
)

// AppError describes a structured application error.
type AppError struct {
	Code    Code
	Message string
	Err     error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped error for errors.Is checks.
func (e *AppError) Unwrap() error {
	return e.Err
}

// New constructs a new AppError with the provided code and message.
func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// Wrap associates an existing error with an application-specific code and message.
func Wrap(err error, code Code, message string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{Code: code, Message: message, Err: err}
}
