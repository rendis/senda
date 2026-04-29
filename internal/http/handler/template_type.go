package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/apperr"
	"github.com/rendis/senda/pkg/slug"
)

type templateTypeResolvedTemplateInvalidator interface {
	InvalidateResolvedTemplates(ctx context.Context, workspaceID uuid.UUID)
	InvalidateAllResolvedTemplates(ctx context.Context)
}

// TemplateTypeHandler handles CRUD operations for template types.
type TemplateTypeHandler struct {
	svc                 *service.TemplateTypeService
	accessSvc           *service.AdapterAccessService
	tsStore             port.TenantStore
	wsStore             port.WorkspaceStore
	templateInvalidator templateTypeResolvedTemplateInvalidator
}

// NewTemplateTypeHandler creates a new TemplateTypeHandler.
func NewTemplateTypeHandler(svc *service.TemplateTypeService, ts port.TenantStore, ws port.WorkspaceStore, invalidator templateTypeResolvedTemplateInvalidator) *TemplateTypeHandler {
	return &TemplateTypeHandler{svc: svc, tsStore: ts, wsStore: ws, templateInvalidator: invalidator}
}

// SetAdapterAccessService wires workspace adapter sharing validation without changing every constructor call site.
func (h *TemplateTypeHandler) SetAdapterAccessService(accessSvc *service.AdapterAccessService) {
	h.accessSvc = accessSvc
}

// Create handles POST /tenants/:tenant_code/workspaces/:workspace_code/template-types.
func (h *TemplateTypeHandler) Create(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	if !ws.IsSystem {
		systemWorkspace, err := resolveSystemWorkspace(c.Request().Context(), ws, h.wsStore)
		if err != nil {
			return mapStoreError(c, err)
		}
		if !effectiveWorkspacePolicies(systemWorkspace).AllowWorkspaceLocalTemplates {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_TEMPLATES_DISABLED", "workspace local templates are disabled by tenant policy")
		}
	}

	return h.create(c, ws)
}

// CreateGlobal handles POST /global/template-types.
func (h *TemplateTypeHandler) CreateGlobal(c *echo.Context) error {
	return h.create(c, nil)
}

func (h *TemplateTypeHandler) create(c *echo.Context, workspace *domain.Workspace) error {
	var req request.CreateTemplateTypeRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	var fieldErrors []response.FieldError
	if err := slug.Validate(req.Slug); err != nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "slug", Message: err.Error()})
	}
	if req.Name == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
	} else if len(req.Name) > 255 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must be at most 255 characters"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	adapterID, err := parseOptionalUUID(req.AdapterID)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "adapter_id", Message: "must be a valid UUID"},
		)
	}

	senderIdentityID, err := parseOptionalUUID(req.SenderIdentityID)
	if err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "sender_identity_id", Message: "must be a valid UUID"},
		)
	}

	var variableSchema map[string]any
	if len(req.VariableSchema) > 0 {
		if err := json.Unmarshal(req.VariableSchema, &variableSchema); err != nil {
			return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
				response.FieldError{Field: "variable_schema", Message: "must be valid JSON"},
			)
		}
	}

	testRecipientMode, testRecipientAddresses, policyFieldErrors := parseTemplateTypeTestRecipientPolicyForCreate(workspace, req)
	if len(policyFieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", policyFieldErrors...)
	}

	var workspaceID *uuid.UUID
	if workspace != nil {
		workspaceID = &workspace.ID
	}

	if h.accessSvc != nil {
		if err := h.accessSvc.ValidateTemplateTypeSelection(c.Request().Context(), workspace, adapterID, senderIdentityID); err != nil {
			return mapTemplateTypeAccessError(c, err)
		}
	}

	tt, err := h.svc.Create(
		c.Request().Context(),
		req.Slug,
		req.Name,
		req.Description,
		adapterID,
		senderIdentityID,
		variableSchema,
		testRecipientMode,
		testRecipientAddresses,
		workspaceID,
	)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.NewTemplateTypeResponse(tt))
}

// Delete handles DELETE /tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug.
func (h *TemplateTypeHandler) Delete(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.deleteType(c, ws)
}

// DeleteGlobal handles DELETE /global/template-types/:slug.
func (h *TemplateTypeHandler) DeleteGlobal(c *echo.Context) error {
	return h.deleteType(c, nil)
}

func (h *TemplateTypeHandler) deleteType(c *echo.Context, workspace *domain.Workspace) error {
	slugParam := c.Param("slug")
	ctx := c.Request().Context()

	var (
		tt              *domain.TemplateType
		workspaceID     *uuid.UUID
		systemWorkspace *domain.Workspace
		err             error
	)
	if workspace == nil {
		tt, err = h.svc.FindBySlugInScope(ctx, slugParam, nil)
	} else {
		workspaceID = &workspace.ID
		var chain []uuid.NullUUID
		chain, systemWorkspace, err = workspaceResolutionChain(ctx, workspace, h.wsStore)
		if err == nil {
			tt, err = h.svc.GetBySlug(ctx, slugParam, chain)
		}
	}
	if err != nil {
		return mapStoreError(c, err)
	}
	if workspace != nil {
		annotateTemplateTypeScope(tt, workspace, systemWorkspace)
		if !isOwnedByCurrentWorkspace(tt.WorkspaceID, workspace) {
			return writePolicyForbidden(c, "READ_ONLY_INHERITED_TEMPLATE_TYPE", "inherited template types are read-only in workspace scope")
		}
		if !workspace.IsSystem && !effectiveWorkspacePolicies(systemWorkspace).AllowWorkspaceLocalTemplates {
			return writePolicyForbidden(c, "WORKSPACE_LOCAL_TEMPLATES_DISABLED", "workspace local templates are disabled by tenant policy")
		}
	}

	if err := h.svc.DeleteType(ctx, tt.ID); err != nil {
		return mapStoreError(c, err)
	}
	if workspace != nil {
		workspaceID = &workspace.ID
	}
	h.invalidateResolvedTemplates(c.Request().Context(), workspaceID)

	return c.NoContent(http.StatusNoContent)
}

// Update handles PUT /tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug.
func (h *TemplateTypeHandler) Update(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	return h.update(c, ws)
}

// UpdateGlobal handles PUT /global/template-types/:slug.
func (h *TemplateTypeHandler) UpdateGlobal(c *echo.Context) error {
	return h.update(c, nil)
}

func (h *TemplateTypeHandler) update(c *echo.Context, workspace *domain.Workspace) error {
	slugParam := c.Param("slug")
	ctx := c.Request().Context()

	tt, _, err := h.loadTemplateTypeForUpdate(ctx, workspace, slugParam)
	if err != nil {
		return mapStoreError(c, err)
	}

	var req request.UpdateTemplateTypeRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	fieldErrors := validateTemplateTypeUpdateRequest(req)
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	previousSlug := tt.Slug
	if field, err := applyTemplateTypeUpdateRequest(tt, req, workspace); err != nil {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid "+field)
	}

	if h.accessSvc != nil {
		if err := h.accessSvc.ValidateTemplateTypeSelection(ctx, workspace, tt.AdapterID, tt.SenderIdentityID); err != nil {
			return mapTemplateTypeAccessError(c, err)
		}
	}

	if err := h.svc.Update(ctx, tt, previousSlug); err != nil {
		return mapStoreError(c, err)
	}
	h.invalidateResolvedTemplates(ctx, workspaceIDFor(workspace))

	return c.JSON(http.StatusOK, response.NewTemplateTypeResponse(tt))
}

func (h *TemplateTypeHandler) loadTemplateTypeForUpdate(
	ctx context.Context,
	workspace *domain.Workspace,
	slugParam string,
) (*domain.TemplateType, *domain.Workspace, error) {
	if workspace == nil {
		tt, err := h.svc.FindBySlugInScope(ctx, slugParam, nil)
		return tt, nil, err
	}

	chain, systemWorkspace, err := workspaceResolutionChain(ctx, workspace, h.wsStore)
	if err != nil {
		return nil, nil, err
	}

	tt, err := h.svc.GetBySlug(ctx, slugParam, chain)
	if err != nil {
		return nil, nil, err
	}
	annotateTemplateTypeScope(tt, workspace, systemWorkspace)

	if !isOwnedByCurrentWorkspace(tt.WorkspaceID, workspace) {
		return nil, nil, apperr.Forbidden("inherited template types are read-only in workspace scope")
	}
	if !workspace.IsSystem && !effectiveWorkspacePolicies(systemWorkspace).AllowWorkspaceLocalTemplates {
		return nil, nil, apperr.Forbidden("workspace local templates are disabled by tenant policy")
	}

	return tt, systemWorkspace, nil
}

func validateTemplateTypeUpdateRequest(req request.UpdateTemplateTypeRequest) []response.FieldError {
	var fieldErrors []response.FieldError
	if req.Slug != nil {
		if err := slug.Validate(*req.Slug); err != nil {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "slug", Message: err.Error()})
		}
	}
	if req.Name != nil {
		if *req.Name == "" {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "is required"})
		} else if len(*req.Name) > 255 {
			fieldErrors = append(fieldErrors, response.FieldError{Field: "name", Message: "must be at most 255 characters"})
		}
	}
	return fieldErrors
}

func applyTemplateTypeUpdateRequest(tt *domain.TemplateType, req request.UpdateTemplateTypeRequest, workspace *domain.Workspace) (string, error) { //nolint:gocognit // request patching mixes optional field updates with environment-aware validation
	if req.Slug != nil {
		tt.Slug = *req.Slug
	}
	if req.Name != nil {
		tt.Name = *req.Name
	}
	if req.AdapterID != nil {
		if *req.AdapterID == "" {
			tt.AdapterID = nil
			tt.SenderIdentityID = nil
		} else {
			adapterID, err := parseOptionalUUID(req.AdapterID)
			if err != nil {
				return "adapter_id", err
			}
			tt.AdapterID = adapterID
		}
	}
	if req.SenderIdentityID != nil {
		if *req.SenderIdentityID == "" {
			tt.SenderIdentityID = nil
		} else {
			senderIdentityID, err := parseOptionalUUID(req.SenderIdentityID)
			if err != nil {
				return "sender_identity_id", err
			}
			tt.SenderIdentityID = senderIdentityID
		}
	}
	if req.TestRecipientMode != nil || req.TestRecipientAddresses != nil {
		if workspace == nil || workspace.Environment != domain.EnvironmentTest {
			return "test_recipient_mode", domain.ErrValidation
		}
		if req.TestRecipientMode != nil {
			trimmed := strings.TrimSpace(*req.TestRecipientMode)
			if trimmed == "" {
				tt.TestRecipientMode = nil
			} else {
				mode := domain.TestRecipientMode(trimmed)
				if !mode.Valid() {
					return "test_recipient_mode", domain.ErrValidation
				}
				tt.TestRecipientMode = &mode
			}
		}
		if req.TestRecipientAddresses != nil {
			if len(*req.TestRecipientAddresses) == 0 {
				tt.TestRecipientAddresses = nil
				return "", nil
			}
			addresses := domain.NormalizeRecipientAddresses(*req.TestRecipientAddresses)
			if err := domain.ValidateRecipientAddresses(addresses); err != nil {
				return "test_recipient_addresses", err
			}
			tt.TestRecipientAddresses = addresses
		}
	}
	return "", nil
}

func parseTemplateTypeTestRecipientPolicyForCreate(
	workspace *domain.Workspace,
	req request.CreateTemplateTypeRequest,
) (*domain.TestRecipientMode, []string, []response.FieldError) {
	if req.TestRecipientMode == nil && len(req.TestRecipientAddresses) == 0 {
		return nil, nil, nil
	}
	if workspace == nil || workspace.Environment != domain.EnvironmentTest {
		return nil, nil, []response.FieldError{
			{Field: "test_recipient_mode", Message: "is only supported in the test environment"},
		}
	}

	mode := domain.TestRecipientModeReplace
	if req.TestRecipientMode != nil {
		mode = domain.TestRecipientMode(strings.TrimSpace(*req.TestRecipientMode))
		if !mode.Valid() {
			return nil, nil, []response.FieldError{
				{Field: "test_recipient_mode", Message: "must be replace or append"},
			}
		}
	}

	addresses := domain.NormalizeRecipientAddresses(req.TestRecipientAddresses)
	if err := domain.ValidateRecipientAddresses(addresses); err != nil {
		return nil, nil, []response.FieldError{
			{Field: "test_recipient_addresses", Message: "must contain valid email addresses"},
		}
	}

	return &mode, addresses, nil
}

func workspaceIDFor(workspace *domain.Workspace) *uuid.UUID {
	if workspace == nil {
		return nil
	}
	return &workspace.ID
}

func mapTemplateTypeAccessError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrAdapterAccessDenied):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "adapter_id", Message: "is not accessible from this workspace"},
		)
	case errors.Is(err, domain.ErrSenderIdentityRequired):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "sender_identity_id", Message: "is required for shared adapter sender identities"},
		)
	case errors.Is(err, domain.ErrSenderIdentityAccessDenied), errors.Is(err, domain.ErrIdentityNotFound):
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed",
			response.FieldError{Field: "sender_identity_id", Message: "is not accessible from this workspace"},
		)
	default:
		return mapStoreError(c, err)
	}
}

func (h *TemplateTypeHandler) invalidateResolvedTemplates(ctx context.Context, wsID *uuid.UUID) {
	if h.templateInvalidator == nil {
		return
	}
	if wsID == nil {
		h.templateInvalidator.InvalidateAllResolvedTemplates(ctx)
		return
	}
	h.templateInvalidator.InvalidateResolvedTemplates(ctx, *wsID)
}

// Get handles GET /tenants/:tenant_code/workspaces/:workspace_code/template-types/:slug.
func (h *TemplateTypeHandler) Get(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.get(c, &ws.ID)
}

// GetGlobal handles GET /global/template-types/:slug.
func (h *TemplateTypeHandler) GetGlobal(c *echo.Context) error {
	return h.get(c, nil)
}

func (h *TemplateTypeHandler) get(c *echo.Context, workspaceID *uuid.UUID) error {
	slugParam := c.Param("slug")

	if workspaceID == nil {
		tt, err := h.svc.FindBySlugInScope(c.Request().Context(), slugParam, nil)
		if err != nil {
			return mapStoreError(c, err)
		}
		return c.JSON(http.StatusOK, response.NewTemplateTypeResponse(tt))
	}

	workspace, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	chain, systemWorkspace, err := workspaceResolutionChain(c.Request().Context(), workspace, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}
	tt, err := h.svc.GetBySlug(c.Request().Context(), slugParam, chain)
	if err != nil {
		return mapStoreError(c, err)
	}
	annotateTemplateTypeScope(tt, workspace, systemWorkspace)

	return c.JSON(http.StatusOK, response.NewTemplateTypeResponse(tt))
}

// List handles GET /tenants/:tenant_code/workspaces/:workspace_code/template-types.
func (h *TemplateTypeHandler) List(c *echo.Context) error {
	ws, err := resolveWorkspace(c, h.tsStore, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	return h.listTypes(c, ws)
}

// ListGlobal handles GET /global/template-types.
func (h *TemplateTypeHandler) ListGlobal(c *echo.Context) error {
	return h.listTypes(c, nil)
}

func (h *TemplateTypeHandler) listTypes(c *echo.Context, workspace *domain.Workspace) error {
	opts := pagination.ParseListOptions(c)
	ctx := c.Request().Context()

	if workspace == nil {
		types, nextCursor, err := h.svc.ListTypes(ctx, nil, opts)
		if err != nil {
			return mapStoreError(c, err)
		}

		items := make([]response.TemplateTypeResponse, len(types))
		for i, tt := range types {
			items[i] = response.NewTemplateTypeResponse(tt)
		}

		return c.JSON(http.StatusOK, response.TemplateTypeListResponse{
			Items:      items,
			NextCursor: nextCursor,
			HasMore:    nextCursor != "",
		})
	}

	systemWorkspace, err := resolveSystemWorkspace(ctx, workspace, h.wsStore)
	if err != nil {
		return mapStoreError(c, err)
	}

	visible := make([]*domain.TemplateType, 0)
	seen := make(map[string]struct{})
	appendUnique := func(list []*domain.TemplateType) {
		for _, tt := range list {
			if _, ok := seen[tt.Slug]; ok {
				continue
			}
			annotateTemplateTypeScope(tt, workspace, systemWorkspace)
			visible = append(visible, tt)
			seen[tt.Slug] = struct{}{}
		}
	}

	localTypes, _, err := h.svc.ListTypes(ctx, &workspace.ID, opts)
	if err != nil {
		return mapStoreError(c, err)
	}
	appendUnique(localTypes)

	if systemWorkspace != nil && systemWorkspace.ID != workspace.ID {
		systemTypes, _, err := h.svc.ListTypes(ctx, &systemWorkspace.ID, opts)
		if err != nil {
			return mapStoreError(c, err)
		}
		appendUnique(systemTypes)
	}

	items := make([]response.TemplateTypeResponse, len(visible))
	for i, tt := range visible {
		items[i] = response.NewTemplateTypeResponse(tt)
	}

	return c.JSON(http.StatusOK, response.TemplateTypeListResponse{
		Items:      items,
		NextCursor: "",
		HasMore:    false,
	})
}
