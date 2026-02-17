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

	// Template resolution errors (HT-11)
	ErrTemplateTypeNotFound = errors.New("template type not found")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrTemplateDisabled     = errors.New("template is disabled")
	ErrNoPublishedVersion   = errors.New("no published version")
	ErrDomainNotVerified    = errors.New("domain not verified")

	// Domain errors (HT-12)
	ErrDomainNotFound = errors.New("domain not found")

	// Pagination errors (HT-19)
	ErrInvalidCursor = errors.New("invalid cursor")
)
