package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
)

type resolvedTemplateInvalidator interface {
	InvalidateResolvedTemplates(ctx context.Context, workspaceID uuid.UUID)
	InvalidateAllResolvedTemplates(ctx context.Context)
}

// TemplateHandler handles template, version, locale CRUD, and MJML preview.
type TemplateHandler struct {
	svc                 *service.TemplateService
	store               port.TemplateStore
	tsStore             port.TenantStore
	wsStore             port.WorkspaceStore
	testSendSvc         *service.TestSendService
	templateInvalidator resolvedTemplateInvalidator
}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler(
	svc *service.TemplateService,
	store port.TemplateStore,
	ts port.TenantStore,
	ws port.WorkspaceStore,
	testSendSvc *service.TestSendService,
	templateInvalidator resolvedTemplateInvalidator,
) *TemplateHandler {
	return &TemplateHandler{
		svc:                 svc,
		store:               store,
		tsStore:             ts,
		wsStore:             ws,
		testSendSvc:         testSendSvc,
		templateInvalidator: normalizeResolvedTemplateInvalidator(templateInvalidator),
	}
}

func normalizeResolvedTemplateInvalidator(invalidator resolvedTemplateInvalidator) resolvedTemplateInvalidator {
	if invalidator == nil {
		return nil
	}

	value := reflect.ValueOf(invalidator)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		if value.IsNil() {
			return nil
		}
	}

	return invalidator
}

// CreateTemplate handles POST .../templates.
func (h *TemplateHandler) CreateTemplate(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.createTemplate(c, &ws.ID)
}

// CreateTemplateGlobal handles POST /global/templates.
func (h *TemplateHandler) CreateTemplateGlobal(c *echo.Context) error {
	return h.createTemplate(c, nil)
}

func (h *TemplateHandler) createTemplate(c *echo.Context, workspaceID *uuid.UUID) error {
	var req request.CreateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.TemplateTypeID == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "template_type_id", Message: "is required"},
		)
	}

	typeID, err := uuid.Parse(req.TemplateTypeID)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "template_type_id", Message: "must be a valid UUID"},
		)
	}

	tpl, err := h.svc.CreateTemplate(c.Request().Context(), typeID, workspaceID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTemplateResponse(tpl))
}

// ListByTemplateType handles GET .../template-types/:slug/templates.
func (h *TemplateHandler) ListByTemplateType(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.listByTemplateType(c, &ws.ID)
}

// ListByTemplateTypeGlobal handles GET /global/template-types/:slug/templates.
func (h *TemplateHandler) ListByTemplateTypeGlobal(c *echo.Context) error {
	return h.listByTemplateType(c, nil)
}

func (h *TemplateHandler) listByTemplateType(c *echo.Context, wsID *uuid.UUID) error {
	slugParam := c.Param("slug")

	tt, err := h.store.FindTypeBySlugInScope(c.Request().Context(), slugParam, wsID)
	if err != nil {
		return mapStoreError(c, err)
	}

	opts := pagination.ParseListOptions(c)

	templates, nextCursor, err := h.svc.ListByType(c.Request().Context(), tt.ID, wsID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}

	items := make([]response.TemplateResponse, len(templates))
	for i, tpl := range templates {
		items[i] = response.NewTemplateResponse(tpl)
	}

	return c.JSON(http.StatusOK, response.TemplateListResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    nextCursor != "",
	})
}

// DisableTemplate handles POST .../templates/:template_id/disable.
func (h *TemplateHandler) DisableTemplate(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.setTemplateDisabled(c, &ws.ID, true)
}

// EnableTemplate handles POST .../templates/:template_id/enable.
func (h *TemplateHandler) EnableTemplate(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.setTemplateDisabled(c, &ws.ID, false)
}

// DisableTemplateGlobal handles POST /global/templates/:template_id/disable.
func (h *TemplateHandler) DisableTemplateGlobal(c *echo.Context) error {
	return h.setTemplateDisabled(c, nil, true)
}

// EnableTemplateGlobal handles POST /global/templates/:template_id/enable.
func (h *TemplateHandler) EnableTemplateGlobal(c *echo.Context) error {
	return h.setTemplateDisabled(c, nil, false)
}

func (h *TemplateHandler) setTemplateDisabled(c *echo.Context, wsID *uuid.UUID, disabled bool) error {
	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template ID")
	}

	if disabled {
		if err := h.svc.DisableTemplate(c.Request().Context(), templateID, wsID); err != nil {
			return mapTemplateError(c, err)
		}
	} else {
		if err := h.svc.EnableTemplate(c.Request().Context(), templateID, wsID); err != nil {
			return mapTemplateError(c, err)
		}
	}
	h.invalidateResolvedTemplates(c.Request().Context(), wsID)

	return c.NoContent(http.StatusNoContent)
}

func (h *TemplateHandler) invalidateResolvedTemplates(ctx context.Context, wsID *uuid.UUID) {
	if h.templateInvalidator == nil {
		return
	}
	if wsID == nil {
		h.templateInvalidator.InvalidateAllResolvedTemplates(ctx)
		return
	}
	h.templateInvalidator.InvalidateResolvedTemplates(ctx, *wsID)
}

// GetVersion handles GET .../templates/:template_id/versions/:version_id.
func (h *TemplateHandler) GetVersion(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	ver, err := h.svc.GetVersionByID(c.Request().Context(), versionID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTemplateVersionResponse(ver))
}

// UpdateVersion handles PUT .../templates/:template_id/versions/:version_id.
func (h *TemplateHandler) UpdateVersion(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	var req request.CreateVersionRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	ver, err := h.svc.GetVersionByID(c.Request().Context(), versionID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	if ver.Status != domain.VersionStatusDraft {
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "only draft versions can be updated")
	}

	if req.Subject != "" {
		ver.Subject = req.Subject
	}
	if req.PreviewText != "" {
		ver.PreviewText = req.PreviewText
	}
	if req.FromName != "" {
		ver.FromName = req.FromName
	}
	if req.ReplyTo != nil {
		ver.ReplyTo = req.ReplyTo
	}
	if req.BodyMJML != "" {
		ver.BodyMJML = req.BodyMJML
	}
	if req.DefaultLocale != "" {
		ver.DefaultLocale = req.DefaultLocale
	}
	if len(req.EditorData) > 0 {
		var editorData map[string]any
		if err := json.Unmarshal(req.EditorData, &editorData); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "editor_data", Message: "must be valid JSON"},
			)
		}
		ver.EditorData = editorData
	}

	if err := h.svc.UpdateVersion(c.Request().Context(), ver); err != nil {
		return mapTemplateError(c, err)
	}

	// Re-fetch to get updated_at from DB.
	updated, err := h.svc.GetVersionByID(c.Request().Context(), versionID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTemplateVersionResponse(updated))
}

// TestSend handles POST .../templates/:template_id/test-send.
func (h *TemplateHandler) TestSend(c *echo.Context) error {
	if h.testSendSvc == nil {
		return response.WriteError(c, http.StatusNotImplemented, "NOT_IMPLEMENTED", "test send not configured")
	}

	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template ID")
	}

	var req struct {
		RecipientEmail string         `json:"recipient_email"`
		Variables      map[string]any `json:"variables"`
		Locale         *string        `json:"locale"`
	}
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	if req.RecipientEmail == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "recipient_email is required")
	}

	result, err := h.testSendSvc.Send(c.Request().Context(), &service.TestSendRequest{
		TemplateID:     templateID,
		RecipientEmail: req.RecipientEmail,
		Variables:      req.Variables,
		Locale:         req.Locale,
	})
	if err != nil {
		return mapTestSendError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":              "sent",
		"provider_message_id": result.ProviderMessageID,
		"from":                result.FromAddress,
	})
}

func mapTestSendError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrNoPublishedVersion):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_PUBLISHED_VERSION", "no published version")
	case errors.Is(err, domain.ErrNoAdapterConfigured):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_ADAPTER", "no adapter assigned to template type")
	case errors.Is(err, domain.ErrNoDefaultIdentity):
		return response.WriteError(c, http.StatusUnprocessableEntity, "NO_DEFAULT_IDENTITY", "no sender identity configured")
	case errors.Is(err, domain.ErrNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	default:
		return response.WriteError(c, http.StatusUnprocessableEntity, "SEND_FAILED", fmt.Sprintf("test send failed: %v", err))
	}
}

// DeleteTemplate handles DELETE .../templates/:template_id.
func (h *TemplateHandler) DeleteTemplate(c *echo.Context) error {
	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template ID")
	}
	if err := h.svc.DeleteTemplate(c.Request().Context(), templateID); err != nil {
		if errors.Is(err, domain.ErrHasPublishedVersion) {
			return response.WriteError(c, http.StatusConflict, "HAS_PUBLISHED_VERSION", "cannot delete template with a published version")
		}
		return mapTemplateError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteVersion handles DELETE .../templates/:template_id/versions/:version_id.
func (h *TemplateHandler) DeleteVersion(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}
	if err := h.svc.DeleteVersion(c.Request().Context(), versionID); err != nil {
		if errors.Is(err, domain.ErrVersionNotDraft) {
			return response.WriteError(c, http.StatusConflict, "VERSION_NOT_DRAFT", "only draft versions can be deleted")
		}
		return mapTemplateError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// ListVersions handles GET .../templates/:template_id/versions.
func (h *TemplateHandler) ListVersions(c *echo.Context) error {
	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template ID")
	}

	versions, err := h.svc.ListVersions(c.Request().Context(), templateID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTemplateVersionListResponse(versions))
}

// CreateVersion handles POST .../templates/:template_id/versions.
func (h *TemplateHandler) CreateVersion(c *echo.Context) error {
	templateID, err := uuid.Parse(c.Param("template_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid template ID")
	}

	var req request.CreateVersionRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if req.Subject == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "subject", Message: "is required"})
	}
	if req.FromName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "from_name", Message: "is required"})
	}
	if req.BodyMJML == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "body_mjml", Message: "is required"})
	}
	if req.DefaultLocale == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "default_locale", Message: "is required"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	var editorData map[string]any
	if len(req.EditorData) > 0 {
		if err := json.Unmarshal(req.EditorData, &editorData); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "editor_data", Message: "must be valid JSON"},
			)
		}
	}

	ver, err := h.svc.CreateVersion(c.Request().Context(), templateID,
		req.Subject, req.PreviewText, req.FromName,
		req.ReplyTo, req.BodyMJML, req.DefaultLocale, editorData, nil,
	)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTemplateVersionResponse(ver))
}

// PublishVersion handles POST .../templates/:template_id/versions/:version_id/publish.
func (h *TemplateHandler) PublishVersion(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	if err := h.svc.PublishVersion(c.Request().Context(), versionID); err != nil {
		return mapTemplateError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// SetLocale handles POST .../templates/:template_id/versions/:version_id/locales.
func (h *TemplateHandler) SetLocale(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	locale := c.Param("locale")
	if locale == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "locale is required")
	}

	var req request.SetLocaleRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var editorData map[string]any
	if len(req.EditorData) > 0 {
		if err := json.Unmarshal(req.EditorData, &editorData); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "editor_data", Message: "must be valid JSON"},
			)
		}
	}

	loc, err := h.svc.SetLocale(c.Request().Context(), versionID, locale,
		req.Subject, req.PreviewText, req.FromName, req.BodyMJML, editorData,
	)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTemplateVersionLocaleResponse(loc))
}

// UpdateLocale handles PUT .../templates/:template_id/versions/:version_id/locales/:locale.
func (h *TemplateHandler) UpdateLocale(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	locale := c.Param("locale")
	if locale == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "locale is required")
	}

	var req request.SetLocaleRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var editorData map[string]any
	if len(req.EditorData) > 0 {
		if err := json.Unmarshal(req.EditorData, &editorData); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "editor_data", Message: "must be valid JSON"},
			)
		}
	}

	loc, err := h.svc.SetLocale(c.Request().Context(), versionID, locale,
		req.Subject, req.PreviewText, req.FromName, req.BodyMJML, editorData,
	)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTemplateVersionLocaleResponse(loc))
}

// GetLocale handles GET .../templates/:template_id/versions/:version_id/locales/:locale.
func (h *TemplateHandler) GetLocale(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	locale := c.Param("locale")
	if locale == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "locale is required")
	}

	loc, err := h.svc.GetLocale(c.Request().Context(), versionID, locale)
	if err != nil {
		return mapTemplateError(c, err)
	}

	return c.JSON(http.StatusOK, response.NewTemplateVersionLocaleResponse(loc))
}

// ListLocales handles GET .../templates/:template_id/versions/:version_id/locales.
func (h *TemplateHandler) ListLocales(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	locales, err := h.svc.ListLocales(c.Request().Context(), versionID)
	if err != nil {
		return mapTemplateError(c, err)
	}

	items := make([]response.TemplateVersionLocaleResponse, len(locales))
	for i, loc := range locales {
		items[i] = response.NewTemplateVersionLocaleResponse(loc)
	}

	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// DeleteLocale handles DELETE .../templates/:template_id/versions/:version_id/locales/:locale.
func (h *TemplateHandler) DeleteLocale(c *echo.Context) error {
	versionID, err := uuid.Parse(c.Param("version_id"))
	if err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid version ID")
	}

	locale := c.Param("locale")
	if locale == "" {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "locale is required")
	}

	if err := h.svc.DeleteLocale(c.Request().Context(), versionID, locale); err != nil {
		return mapTemplateError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// mapTemplateError maps template-specific domain errors to HTTP responses.
// Extends mapStoreError with template/version/locale-specific error codes
// that the generic mapStoreError (in tenant.go) does not cover.
func mapTemplateError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrTemplateTypeNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "template type not found")
	case errors.Is(err, domain.ErrTemplateNotFound):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "template not found")
	case errors.Is(err, domain.ErrTemplateDisabled):
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "template is disabled")
	case errors.Is(err, domain.ErrNoPublishedVersion):
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "no published version")
	case errors.Is(err, domain.ErrDomainNotVerified):
		return response.WriteError(c, http.StatusConflict, "CONFLICT", "domain not verified")
	default:
		return mapStoreError(c, err)
	}
}

// PreviewMJML handles POST .../templates/:template_id/preview-mjml.
func (h *TemplateHandler) PreviewMJML(c *echo.Context) error {
	var req request.MJMLPreviewRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	if req.MJML == "" {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "mjml", Message: "is required"},
		)
	}

	html, err := h.svc.PreviewMJML(c.Request().Context(), req.MJML)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	}

	return c.JSON(http.StatusOK, response.MJMLPreviewResponse{HTML: html})
}
