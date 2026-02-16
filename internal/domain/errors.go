package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrConflict            = errors.New("conflict")
	ErrValidation          = errors.New("validation error")
	ErrForbidden           = errors.New("forbidden")
	ErrNoAdapterConfigured = errors.New("no adapter configured")
	ErrInvalidRef          = errors.New("invalid template reference")
	ErrSuppressed          = errors.New("recipient is suppressed")
	ErrRateLimited         = errors.New("rate limited")
	ErrMaintenanceMode     = errors.New("maintenance mode")
)
