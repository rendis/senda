package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/service"
)

type sendService interface {
	Send(ctx context.Context, req *service.SendRequest) (*service.SendResponse, error)
	SendBatch(ctx context.Context, req *service.SendBatchRequest) (*service.SendBatchResponse, error)
}

// SendHandler handles the data-plane send endpoint (API Key auth).
type SendHandler struct {
	sendService   sendService
	batchMaxItems int
}

// NewSendHandler creates a new SendHandler.
func NewSendHandler(ss sendService, batchMaxItems int) *SendHandler {
	return &SendHandler{sendService: ss, batchMaxItems: batchMaxItems}
}

// Send handles POST /api/v1/send.
// Accepts API Key auth -- workspace_id is resolved from the API key context.
func (h *SendHandler) Send(c *echo.Context) error {
	// Extract workspace_id from API key auth context.
	// The auth middleware sets this for API key-authenticated requests.
	wsID, ok := c.Get(middleware.ContextKeyWorkspaceID).(uuid.UUID)
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
	if len(req.To) > 50 {
		slog.Warn("send rejected", "reason", "to_limit_exceeded", "recipient_count", len(req.To), "workspace_id", wsID)
		fieldErrors = append(fieldErrors, response.FieldError{Field: "to", Message: "must contain at most 50 recipients"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	// Extract HTTP headers for code injectors.
	headers := make(map[string]string)
	for k, vals := range c.Request().Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	svcReq := &service.SendRequest{
		Ref:             req.Ref,
		To:              req.To,
		CC:              req.CC,
		BCC:             req.BCC,
		Variables:       req.Variables,
		ExternalID:      req.ExternalID,
		Locale:          req.Locale,
		AuthWorkspaceID: wsID,
		Headers:         headers,
		Source: service.SendSource{
			Type: domain.EmailSourceTypeDataPlaneAPIKey,
		},
	}

	resp, err := h.sendService.Send(c.Request().Context(), svcReq)
	if err != nil {
		return mapSendError(c, err)
	}

	return c.JSON(http.StatusAccepted, response.NewSendEmailResponse(resp))
}

// SendBatch handles POST /api/v1/send/batch.
// Accepts API Key auth -- workspace_id is resolved from the API key context.
func (h *SendHandler) SendBatch(c *echo.Context) error {
	wsID, ok := c.Get(middleware.ContextKeyWorkspaceID).(uuid.UUID)
	if !ok || wsID == uuid.Nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "workspace context required (API key auth)")
	}

	var req request.SendBatchRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Ref == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "ref", Message: "is required"})
	}
	if len(req.Items) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "items", Message: "must contain at least 1 item"})
	}
	if len(req.Items) > h.effectiveBatchMaxItems() {
		fieldErrors = append(fieldErrors, response.FieldError{
			Field:   "items",
			Message: fmt.Sprintf("must contain at most %d items", h.effectiveBatchMaxItems()),
		})
	}
	for i, item := range req.Items {
		if item.To == "" {
			fieldErrors = append(fieldErrors, response.FieldError{
				Field:   fmt.Sprintf("items[%d].to", i),
				Message: "is required",
			})
		}
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	headers := make(map[string]string)
	for k, vals := range c.Request().Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	items := make([]service.SendBatchItemRequest, len(req.Items))
	for i, item := range req.Items {
		items[i] = service.SendBatchItemRequest{
			To:         item.To,
			CC:         item.CC,
			BCC:        item.BCC,
			Variables:  item.Variables,
			ExternalID: item.ExternalID,
			Locale:     item.Locale,
		}
	}

	resp, err := h.sendService.SendBatch(c.Request().Context(), &service.SendBatchRequest{
		Ref:             req.Ref,
		Items:           items,
		AuthWorkspaceID: wsID,
		Headers:         headers,
		Source: service.SendSource{
			Type: domain.EmailSourceTypeDataPlaneAPIKey,
		},
	})
	if err != nil {
		return mapSendError(c, err)
	}

	return c.JSON(http.StatusAccepted, response.NewSendBatchResponse(resp))
}

func (h *SendHandler) effectiveBatchMaxItems() int {
	if h.batchMaxItems > 0 {
		return h.batchMaxItems
	}
	return 100
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
	case errors.Is(err, domain.ErrNoDefaultIdentity):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_DEFAULT_IDENTITY", "no default sender identity configured for this adapter")
	case errors.Is(err, domain.ErrWorkspaceScopeMismatch):
		slog.Warn("send rejected", "reason", "scope_mismatch")
		return response.WriteError(c, http.StatusForbidden, "FORBIDDEN", "API key scope does not match template workspace")
	case errors.Is(err, domain.ErrSystemWorkspaceBlocked):
		slog.Warn("send rejected", "reason", "system_workspace_blocked")
		return response.WriteError(c, http.StatusUnprocessableEntity, "SYSTEM_WORKSPACE_BLOCKED", "system workspace cannot send emails")
	case errors.Is(err, domain.ErrDomainNotVerified):
		return response.WriteError(c, http.StatusUnprocessableEntity, "DOMAIN_NOT_VERIFIED", "from domain is not verified")
	case errors.Is(err, domain.ErrSuppressed):
		return response.WriteError(c, http.StatusUnprocessableEntity, "SUPPRESSED", "recipient is suppressed")
	case errors.Is(err, domain.ErrTemplateDisabled):
		slog.Warn("send rejected", "reason", "template_disabled")
		return response.WriteError(c, http.StatusConflict, "TEMPLATE_DISABLED", "template is disabled")
	case errors.Is(err, domain.ErrNoPublishedVersion):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_PUBLISHED_VERSION", "no published version for this template")
	case errors.Is(err, domain.ErrRateLimited):
		return response.WriteError(c, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
	default:
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
