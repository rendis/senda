package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/request"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/service"
)

// TemplateHandler handles template, version, locale CRUD, and MJML preview.
type TemplateHandler struct {
	svc     *service.TemplateService
	store   port.TemplateStore
	tsStore port.TenantStore
	wsStore port.WorkspaceStore
}

// NewTemplateHandler creates a new TemplateHandler.
func NewTemplateHandler(svc *service.TemplateService, store port.TemplateStore, ts port.TenantStore, ws port.WorkspaceStore) *TemplateHandler {
	return &TemplateHandler{svc: svc, store: store, tsStore: ts, wsStore: ws}
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
	if req.FromEmail == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "from_email", Message: "is required"})
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
		req.Subject, req.PreviewText, req.FromName, req.FromEmail,
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

// DeleteLocale handles DELETE .../templates/:template_id/versions/:version_id/locales/:locale.
// TODO: Requires port.TemplateStore.DeleteLocale(ctx, versionID, locale) to be added.
// Once the store interface is extended, this should soft-delete or hard-delete the locale override.
// Until then, returns 501 Not Implemented.
func (h *TemplateHandler) DeleteLocale(c *echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]string{
		"message": "delete locale not yet implemented",
	})
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
