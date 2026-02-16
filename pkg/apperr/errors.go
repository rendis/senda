package apperr

import (
	"fmt"
	"net/http"
)

// AppError represents an application-level error with HTTP status mapping.
type AppError struct {
	Code    int    // HTTP status
	Message string
	Err     error // wrapped domain error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NotFound(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Conflict(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusConflict,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Forbidden(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusForbidden,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Validation(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: fmt.Sprintf(msg, args...),
	}
}

func BadRequest(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: fmt.Sprintf(msg, args...),
	}
}

func UnprocessableEntity(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusUnprocessableEntity,
		Message: fmt.Sprintf(msg, args...),
	}
}

func Internal(msg string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: fmt.Sprintf(msg, args...),
	}
}

// Wrap sets the wrapped error and returns the receiver for chaining.
func (e *AppError) Wrap(err error) *AppError {
	e.Err = err
	return e
}

// WithErr creates an AppError with the given wrapped error, status code, and message.
func WithErr(err error, code int, msg string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(msg, args...), Err: err}
}
