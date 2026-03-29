package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/service"
	"github.com/rendis/senda/pkg/slug"
)

// OnboardingHandler handles the first-use onboarding endpoints.
type OnboardingHandler struct {
	svc      *service.OnboardingService
	verifier port.OIDCVerifier
}

// NewOnboardingHandler creates a new OnboardingHandler.
func NewOnboardingHandler(svc *service.OnboardingService, verifier port.OIDCVerifier) *OnboardingHandler {
	return &OnboardingHandler{svc: svc, verifier: verifier}
}

// Status handles GET /api/v1/onboarding/status (PUBLIC - no auth).
func (h *OnboardingHandler) Status(c *echo.Context) error {
	needs, err := h.svc.Status(c.Request().Context())
	if err != nil {
		return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}

	return c.JSON(http.StatusOK, response.OnboardingStatusResponse{
		NeedsOnboarding: needs,
	})
}

// Setup handles POST /api/v1/onboarding/setup.
// Requires an OIDC bearer token but NOT an existing member record.
func (h *OnboardingHandler) Setup(c *echo.Context) error {
	// Extract and verify OIDC token directly (not via auth middleware).
	claims, err := h.extractOIDCClaims(c)
	if err != nil {
		return response.WriteError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or missing authentication token")
	}

	var req request.OnboardingSetupRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	// Validate input.
	var fieldErrors []response.FieldError
	if err := slug.Validate(req.TenantCode); err != nil {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "tenant_code", Message: err.Error()})
	}
	if req.TenantName == "" {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "tenant_name", Message: "is required"})
	} else if len(req.TenantName) > 255 {
		fieldErrors = append(fieldErrors, response.FieldError{Field: "tenant_name", Message: "must be at most 255 characters"})
	}
	if len(fieldErrors) > 0 {
		return response.WriteError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "validation failed", fieldErrors...)
	}

	svcReq := &service.OnboardingRequest{
		TenantCode: req.TenantCode,
		TenantName: req.TenantName,
	}

	result, err := h.svc.Setup(c.Request().Context(), claims, svcReq)
	if err != nil {
		return mapStoreError(c, err)
	}

	return c.JSON(http.StatusCreated, response.OnboardingSetupResponse{
		Member: response.OnboardingMemberSummary{
			ID:    result.Member.ID.String(),
			Email: result.Member.Email,
		},
		Tenant: response.OnboardingTenantSummary{
			ID:   result.Tenant.ID.String(),
			Code: result.Tenant.Code,
			Name: result.Tenant.Name,
		},
		Workspace: response.OnboardingWorkspaceSummary{
			ID:   result.Workspace.ID.String(),
			Code: result.Workspace.Code,
			Name: result.Workspace.Name,
		},
	})
}

// extractOIDCClaims extracts the Bearer token from the Authorization header
// and verifies it using the OIDCVerifier.
func (h *OnboardingHandler) extractOIDCClaims(c *echo.Context) (*port.OIDCClaims, error) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return nil, echo.ErrUnauthorized
	}

	token, found := strings.CutPrefix(authHeader, "Bearer ")
	if !found || token == "" {
		return nil, echo.ErrUnauthorized
	}

	return h.verifier.Verify(c.Request().Context(), token)
}
