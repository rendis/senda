package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/netutil"
)

// WebhookHandler handles CRUD operations for webhooks and test dispatch.
type WebhookHandler struct {
	store   port.WebhookStore
	svc     *service.WebhookService
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(ws port.WebhookStore, svc *service.WebhookService, ts port.TenantStore, wss port.WorkspaceStore) *WebhookHandler {
	return &WebhookHandler{store: ws, svc: svc, tsStore: ts, wsStore: wss}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/webhooks.
// Returns the secret ONLY on creation (never again).
func (h *WebhookHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.CreateWebhookRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.URL == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "url", Message: "is required"})
	} else if !isValidWebhookURL(req.URL) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "url", Message: "must be a valid HTTPS URL"})
	}
	if len(req.Events) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "events", Message: "is required"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}

	now := time.Now().UTC()
	wh := &domain.Webhook{
		ID:          uuid.Must(uuid.NewV7()),
		WorkspaceID: ws.ID,
		URL:         req.URL,
		Secret:      secret,
		Events:      req.Events,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.Create(c.Request().Context(), wh); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewWebhookCreatedResponse(wh))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/webhooks.
func (h *WebhookHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	opts := pagination.ParseListOptions(c)

	page, err := h.store.ListByWorkspace(c.Request().Context(), ws.ID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWebhookListResponse(page))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id.
func (h *WebhookHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook ID")
	}

	wh, err := h.store.GetByID(c.Request().Context(), webhookID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if wh.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	return c.JSON(http.StatusOK, response.NewWebhookResponse(wh))
}

// Update handles PUT /tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id.
func (h *WebhookHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook ID")
	}

	ctx := c.Request().Context()
	wh, err := h.store.GetByID(ctx, webhookID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if wh.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	var req request.UpdateWebhookRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.URL != nil {
		if *req.URL == "" {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "url", Message: "cannot be empty"},
			)
		}
		if !isValidWebhookURL(*req.URL) {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "url", Message: "must be a valid HTTPS URL"},
			)
		}
		wh.URL = *req.URL
	}
	if req.Events != nil {
		if len(*req.Events) == 0 {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "events", Message: "cannot be empty"},
			)
		}
		wh.Events = *req.Events
	}
	if req.IsActive != nil {
		wh.IsActive = *req.IsActive
	}

	wh.UpdatedAt = time.Now().UTC()
	if err := h.store.Update(ctx, wh); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewWebhookResponse(wh))
}

// Delete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id.
// Webhooks use hard delete (no hierarchy).
func (h *WebhookHandler) Delete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook ID")
	}

	ctx := c.Request().Context()
	wh, err := h.store.GetByID(ctx, webhookID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if wh.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	if err := h.store.Delete(ctx, webhookID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// Test handles POST /tenants/:tenant_code/workspaces/:workspace_code/webhooks/:id/test.
// Dispatches a webhook.test ping event to the specified webhook.
func (h *WebhookHandler) Test(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	webhookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook ID")
	}

	ctx := c.Request().Context()
	wh, err := h.store.GetByID(ctx, webhookID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if wh.WorkspaceID != ws.ID {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	pingPayload := map[string]string{
		"message":    "webhook test ping",
		"webhook_id": webhookID.String(),
	}

	if err := h.svc.Dispatch(ctx, ws.ID, "webhook.test", pingPayload); err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to dispatch test event")
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "dispatched"})
}

// generateWebhookSecret generates a 32-byte random hex string for HMAC signing.
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isValidWebhookURL validates that the webhook URL is a proper HTTPS URL
// pointing to a public (non-private, non-reserved) host.
func isValidWebhookURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return false
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return false
	}
	// Block private/reserved IPs
	if isPrivateOrReservedHost(hostname) {
		return false
	}
	return true
}

// isPrivateOrReservedHost delegates to the shared netutil.IsPrivateOrReservedHost.
// The shared utility should also be called by webhook_worker.go at delivery time
// to guard against DNS rebinding attacks. The orchestrator will wire it.
func isPrivateOrReservedHost(hostname string) bool {
	return netutil.IsPrivateOrReservedHost(hostname)
}
