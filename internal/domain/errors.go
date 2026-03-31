package domain

import "errors"

var (
	ErrNotFound               = errors.New("not found")
	ErrConflict               = errors.New("conflict")
	ErrValidation             = errors.New("validation error")
	ErrForbidden              = errors.New("forbidden")
	ErrWorkspaceScopeMismatch = errors.New("workspace scope mismatch")
	ErrSystemWorkspaceBlocked = errors.New("system workspace cannot be used for send")
	ErrNoAdapterConfigured    = errors.New("no adapter configured")
	ErrInvalidRef             = errors.New("invalid template reference")
	ErrSuppressed             = errors.New("recipient is suppressed")
	ErrRateLimited            = errors.New("rate limited")
	ErrMaintenanceMode        = errors.New("maintenance mode")

	// Template resolution errors (HT-11)
	ErrTemplateTypeNotFound = errors.New("template type not found")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrTemplateDisabled     = errors.New("template is disabled")
	ErrNoPublishedVersion   = errors.New("no published version")
	ErrDomainNotVerified    = errors.New("domain not verified")

	// Domain errors (HT-12)
	ErrDomainNotFound = errors.New("domain not found")

	// Adapter identity errors
	ErrNoDefaultIdentity   = errors.New("adapter has no default identity")
	ErrIdentityNotFound    = errors.New("adapter identity not found")
	ErrIdentityNotInDomain = errors.New("email not within adapter's verified domains")

	// Delete errors
	ErrHasPublishedVersion = errors.New("template has a published version and cannot be deleted")
	ErrVersionNotDraft     = errors.New("only draft versions can be deleted")

	// Pagination errors (HT-19)
	ErrInvalidCursor = errors.New("invalid cursor")

	// Business-specific errors (send flow)
	ErrTemplateNotPublished = errors.New("template has no published version")
	ErrAdapterDisabled      = errors.New("adapter is disabled")
	ErrRecipientSuppressed  = errors.New("recipient is suppressed")
	ErrProviderRejected     = errors.New("provider permanently rejected the message")
	ErrQuotaExhausted       = errors.New("sending quota exhausted")
)
