package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/service"
)

// SendHandler handles the data-plane send endpoint (API Key auth).
type SendHandler struct {
	sendService *service.SendService
}

// NewSendHandler creates a new SendHandler.
func NewSendHandler(ss *service.SendService) *SendHandler {
	return &SendHandler{sendService: ss}
}

// Send handles POST /api/v1/send.
// Accepts API Key auth -- workspace_id is resolved from the API key context.
func (h *SendHandler) Send(c *echo.Context) error {
	// Extract workspace_id from API key auth context.
	// The auth middleware sets this for API key-authenticated requests.
	wsID, ok := c.Get("workspace_id").(uuid.UUID)
	if !ok || wsID == uuid.Nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "workspace context required (API key auth)")
	}

	var req request.SendEmailRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Ref == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "ref", Message: "is required"})
	}
	if len(req.To) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "to", Message: "is required"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	// TODO: Validate that the workspace in the ref matches the API key's workspace.
	// This requires resolving the tenant/workspace from the ref (tenant:workspace:template)
	// and comparing against wsID. Currently SendService.Send does this resolution
	// internally but doesn't accept a workspaceID parameter for cross-check.
	// When SendRequest gains a WorkspaceID field, pass wsID here for authorization.

	svcReq := &service.SendRequest{
		Ref:        req.Ref,
		To:         req.To,
		CC:         req.CC,
		BCC:        req.BCC,
		Variables:  req.Variables,
		ExternalID: req.ExternalID,
		Locale:     req.Locale,
	}

	resp, err := h.sendService.Send(c.Request().Context(), svcReq)
	if err != nil {
		return mapSendError(c, err)
	}

	return c.JSON(http.StatusAccepted, response.NewSendEmailResponse(resp))
}

// mapSendError maps send-specific domain errors to HTTP error responses.
func mapSendError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRef):
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template reference")
	case errors.Is(err, domain.ErrNotFound),
		errors.Is(err, domain.ErrTemplateTypeNotFound),
		errors.Is(err, domain.ErrTemplateNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	case errors.Is(err, domain.ErrNoAdapterConfigured):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_ADAPTER", "no adapter configured for this template type")
	case errors.Is(err, domain.ErrDomainNotVerified):
		return response.WriteError(c, http.StatusUnprocessableEntity, "DOMAIN_NOT_VERIFIED", "from domain is not verified")
	case errors.Is(err, domain.ErrSuppressed):
		return response.WriteError(c, http.StatusUnprocessableEntity, "SUPPRESSED", "recipient is suppressed")
	case errors.Is(err, domain.ErrTemplateDisabled):
		return response.WriteError(c, http.StatusUnprocessableEntity, "TEMPLATE_DISABLED", "template is disabled")
	case errors.Is(err, domain.ErrNoPublishedVersion):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_PUBLISHED_VERSION", "no published version for this template")
	case errors.Is(err, domain.ErrRateLimited):
		return response.WriteError(c, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
	default:
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
