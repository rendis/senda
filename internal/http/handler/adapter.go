package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

// AdapterHandler handles CRUD operations for email adapters.
type AdapterHandler struct {
	store         port.AdapterStore
	crypto        port.Crypto
	tsStore       port.TenantStore
	wsStore       port.WorkspaceStore
	senderFactory port.SenderFactory
	identityStore port.AdapterIdentityStore
	deprovisioner port.Deprovisioner // nil if tracking not configured
	logger        *slog.Logger
}

// NewAdapterHandler creates a new AdapterHandler.
func NewAdapterHandler(
	as port.AdapterStore,
	crypto port.Crypto,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	sf port.SenderFactory,
	is port.AdapterIdentityStore,
	deprov port.Deprovisioner,
	logger *slog.Logger,
) *AdapterHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdapterHandler{
		store:         as,
		crypto:        crypto,
		tsStore:       ts,
		wsStore:       ws,
		senderFactory: sf,
		identityStore: is,
		deprovisioner: deprov,
		logger:        logger,
	}
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/adapters.
func (h *AdapterHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.create(c, &ws.ID)
}

// CreateGlobal handles POST /global/adapters.
func (h *AdapterHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *AdapterHandler) create(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.CreateAdapterRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	}
	if !isValidAdapterType(req.AdapterType) {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "adapter_type", Message: "must be one of: ses, gmail"})
	}
	if len(req.Config) == 0 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "config", Message: "is required"})
	}
	if len(fieldErrors) == 0 {
		fieldErrors = append(fieldErrors, validateConfig(domain.AdapterType(req.AdapterType), req.Config)...)
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	encrypted, err := h.crypto.Encrypt(req.Config)
	if err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}

	now := time.Now().UTC()
	adapter := &domain.Adapter{
		ID:                 uuid.Must(uuid.NewV7()),
		WorkspaceID:        workspaceID,
		Name:               req.Name,
		AdapterType:        domain.AdapterType(req.AdapterType),
		ConfigEncrypted:    encrypted,
		IsDefault:          req.IsDefault,
		RateLimitPerSecond: req.RateLimitPerSecond,
		ConfigMeta:         extractPublicConfigFields(domain.AdapterType(req.AdapterType), req.Config),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := h.store.Create(c.Request().Context(), adapter); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewAdapterResponse(adapter))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/adapters.
func (h *AdapterHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.list(c, &ws.ID)
}

// ListGlobal handles GET /global/adapters.
func (h *AdapterHandler) ListGlobal(c *echo.Context) error {
	return h.list(c, nil)
}

func (h *AdapterHandler) list(c *echo.Context, workspaceID *uuid.UUID) error {
	opts := pagination.ParseListOptions(c)

	page, err := h.store.ListByWorkspace(c.Request().Context(), workspaceID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterListResponse(page))
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/adapters/:id.
func (h *AdapterHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *AdapterHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	adapter, err := h.store.GetByID(c.Request().Context(), adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	// Verify workspace ownership.
	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	return c.JSON(http.StatusOK, response.NewAdapterResponse(adapter))
}

// Update handles PUT /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.update(c, &ws.ID)
}

// UpdateGlobal handles PUT /global/adapters/:id.
func (h *AdapterHandler) UpdateGlobal(c *echo.Context) error {
	return h.update(c, nil)
}

func (h *AdapterHandler) update(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	ctx := c.Request().Context()
	adapter, err := h.store.GetByID(ctx, adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	var req request.UpdateAdapterRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.Name != nil {
		if *req.Name == "" {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "name", Message: "cannot be empty"},
			)
		}
		adapter.Name = *req.Name
	}
	if req.Config != nil || req.ConfigurationSetName != nil {
		cfgMap, err := h.decryptConfigMap(adapter)
		if err != nil {
			return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		if req.Config != nil {
			var patch map[string]any
			if err := json.Unmarshal(*req.Config, &patch); err != nil {
				return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
			}
			for k, v := range patch {
				if s, ok := v.(string); ok && s == "" {
					continue
				}
				cfgMap[k] = v
			}
		}
		if req.ConfigurationSetName != nil {
			cfgMap["configuration_set_name"] = *req.ConfigurationSetName
		}
		updated, err := json.Marshal(cfgMap)
		if err != nil {
			return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		encrypted, err := h.crypto.Encrypt(updated)
		if err != nil {
			return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		adapter.ConfigEncrypted = encrypted
		adapter.ConfigMeta = extractPublicConfigFields(adapter.AdapterType, updated)
	}
	if req.IsDefault != nil {
		adapter.IsDefault = *req.IsDefault
	}
	if req.RateLimitPerSecond != nil {
		adapter.RateLimitPerSecond = *req.RateLimitPerSecond
	}

	adapter.UpdatedAt = time.Now().UTC()
	if err := h.store.Update(ctx, adapter); err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewAdapterResponse(adapter))
}

// SoftDelete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/adapters/:id.
func (h *AdapterHandler) SoftDelete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.softDelete(c, &ws.ID)
}

// SoftDeleteGlobal handles DELETE /global/adapters/:id.
func (h *AdapterHandler) SoftDeleteGlobal(c *echo.Context) error {
	return h.softDelete(c, nil)
}

func (h *AdapterHandler) softDelete(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	ctx := c.Request().Context()
	adapter, err := h.store.GetByID(ctx, adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	// Best-effort deprovision of AWS resources for SES adapters.
	if adapter.AdapterType == domain.AdapterTypeSES && h.deprovisioner != nil {
		if err := h.safeDeprovision(ctx, adapterID); err != nil {
			h.logger.WarnContext(ctx, "deprovision failed, proceeding with soft delete",
				"adapter_id", adapterID, "error", err)
		}
	}

	if err := h.store.SoftDelete(ctx, adapterID); err != nil {
		return mapStoreError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *AdapterHandler) safeDeprovision(ctx context.Context, adapterID uuid.UUID) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("deprovision panic: %v", recovered)
		}
	}()

	return h.deprovisioner.Deprovision(ctx, adapterID)
}

// ValidateSES handles POST .../adapters/validate-ses.
// Tests AWS credentials and checks permissions without creating any resources.
func (h *AdapterHandler) ValidateSES(c *echo.Context) error {
	var req struct {
		Region         string `json:"region"`
		AccessKeyID    string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		EndpointURL    string `json:"endpoint_url,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	if req.Region == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "region is required")
	}

	cfg := sesadapter.Config{
		Region:         req.Region,
		AccessKeyID:    req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
		EndpointURL:    req.EndpointURL,
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	result, err := sesadapter.ValidateCredentials(ctx, cfg)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", fmt.Sprintf("credential validation failed: %v", err))
	}

	return c.JSON(http.StatusOK, result)
}

// TestConnection handles POST .../adapters/:id/test (workspace scope).
func (h *AdapterHandler) TestConnection(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.testSend(c, &ws.ID)
}

// TestConnectionGlobal handles POST /global/adapters/:id/test.
func (h *AdapterHandler) TestConnectionGlobal(c *echo.Context) error {
	return h.testSend(c, nil)
}

// testSendTimeout is the maximum duration for a synchronous test send.
const testSendTimeout = 30 * time.Second

func (h *AdapterHandler) testSend(c *echo.Context, workspaceID *uuid.UUID) error {
	adapterID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid adapter ID")
	}

	var req request.TestAdapterRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.To == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "to", Message: "is required"})
	}
	if req.Subject == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "subject", Message: "is required"})
	}
	if req.Body == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "body", Message: "is required"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	ctx := c.Request().Context()
	adapter, err := h.store.GetByID(ctx, adapterID)
	if err != nil {
		return mapStoreError(c, err)
	}

	if !sameScope(adapter.WorkspaceID, workspaceID) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	}

	decrypted, err := h.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "CONFIG_ERROR", "adapter config is corrupted — please update the adapter with valid credentials")
	}

	sender, err := h.senderFactory(ctx, adapter, decrypted)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "ADAPTER_ERROR", fmt.Sprintf("failed to create sender: %v", err))
	}

	from := resolution.ResolveFromAddress(ctx, h.identityStore, adapter, decrypted)
	if from.Address == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_DEFAULT_IDENTITY", "no default sender identity and no delegate_email in config")
	}

	msg := &port.OutgoingEmail{
		From: from,
		To:       port.EmailAddress{Address: req.To},
		Subject:  req.Subject,
		BodyHTML: req.Body,
		BodyText: req.Body,
	}

	sendCtx, cancel := context.WithTimeout(ctx, testSendTimeout)
	defer cancel()

	providerMsgID, err := sender.Send(sendCtx, msg)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "SEND_FAILED", fmt.Sprintf("test send failed: %v", err))
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":              "sent",
		"provider_message_id": providerMsgID,
		"from":                from.Address,
	})
}

// decryptConfigMap decrypts the adapter config and unmarshals it into a map.
func (h *AdapterHandler) decryptConfigMap(adapter *domain.Adapter) (map[string]any, error) {
	decrypted, err := h.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(decrypted, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// extractPublicConfigFields extracts non-sensitive fields from raw config JSON for storage in config_meta.
func extractPublicConfigFields(adapterType domain.AdapterType, rawConfig []byte) map[string]string {
	var cfgMap map[string]any
	if json.Unmarshal(rawConfig, &cfgMap) != nil {
		return nil
	}
	meta := make(map[string]string)
	switch adapterType {
	case domain.AdapterTypeSES:
		if v, ok := cfgMap["region"].(string); ok && v != "" {
			meta["region"] = v
		}
	case domain.AdapterTypeGmail:
		if v, ok := cfgMap["delegate_email"].(string); ok && v != "" {
			meta["delegate_email"] = v
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// validateConfig checks that required fields are present for the adapter type.
func validateConfig(adapterType domain.AdapterType, config json.RawMessage) []response.FieldError {
	var cfgMap map[string]any
	if json.Unmarshal(config, &cfgMap) != nil {
		return []response.FieldError{{Field: "config", Message: "must be a valid JSON object"}}
	}
	str := func(key string) string {
		if v, ok := cfgMap[key].(string); ok {
			return v
		}
		return ""
	}

	var errs []response.FieldError
	switch adapterType {
	case domain.AdapterTypeSES:
		if str("region") == "" {
			errs = append(errs, response.FieldError{Field: "config.region", Message: "is required"})
		}
	case domain.AdapterTypeGmail:
		if str("service_account_json") == "" {
			errs = append(errs, response.FieldError{Field: "config.service_account_json", Message: "is required"})
		}
		if str("delegate_email") == "" {
			errs = append(errs, response.FieldError{Field: "config.delegate_email", Message: "is required"})
		}
	}
	return errs
}

func isValidAdapterType(t string) bool {
	switch domain.AdapterType(t) {
	case domain.AdapterTypeSES, domain.AdapterTypeGmail:
		return true
	}
	return false
}

// sameScope checks that both pointers are nil or point to the same UUID.
func sameScope(a, b *uuid.UUID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
