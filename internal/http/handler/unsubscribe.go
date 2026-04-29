package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/request"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/service"
)

// unsubscribeService is the narrow interface consumed by UnsubscribeHandler.
type unsubscribeService interface {
	GetContext(ctx context.Context, token string) (*service.UnsubscribeContext, error)
	OneClickOptOut(ctx context.Context, token string) error
	OptOutAll(ctx context.Context, token string) error
	GetPreferences(ctx context.Context, token string) (*service.PreferencesView, error)
	UpdatePreferences(ctx context.Context, token string, changes []service.PreferenceChange) error
	Resubscribe(ctx context.Context, token string) error
}

// UnsubscribeHandler handles the public unsubscribe endpoints (no auth required).
type UnsubscribeHandler struct {
	svc unsubscribeService
}

// NewUnsubscribeHandler creates a new UnsubscribeHandler.
func NewUnsubscribeHandler(svc unsubscribeService) *UnsubscribeHandler {
	return &UnsubscribeHandler{svc: svc}
}

// GetContext handles GET /api/v1/u/:token.
// Returns workspace name, template type, email, and current opt-out state.
func (h *UnsubscribeHandler) GetContext(c *echo.Context) error {
	token := c.Param("token")
	ctx, err := h.svc.GetContext(c.Request().Context(), token)
	if err != nil {
		return mapUnsubscribeError(c, err)
	}
	return c.JSON(http.StatusOK, response.UnsubscribeContextResponse{
		WorkspaceName:    ctx.WorkspaceName,
		TemplateTypeSlug: ctx.TemplateTypeSlug,
		TemplateTypeName: ctx.TemplateTypeName,
		Email:            ctx.Email,
		OptedOutOfType:   ctx.OptedOutOfType,
		OptedOutOfAll:    ctx.OptedOutOfAll,
	})
}

// OneClickOptOut handles POST /api/v1/u/:token.
// RFC 8058 one-click unsubscribe — must be idempotent (returns 200 on repeat calls).
func (h *UnsubscribeHandler) OneClickOptOut(c *echo.Context) error {
	token := c.Param("token")
	if err := h.svc.OneClickOptOut(c.Request().Context(), token); err != nil {
		return mapUnsubscribeError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// OptOutAll handles POST /api/v1/u/:token/all.
// Opts the email address out of all workspace email types.
func (h *UnsubscribeHandler) OptOutAll(c *echo.Context) error {
	token := c.Param("token")
	if err := h.svc.OptOutAll(c.Request().Context(), token); err != nil {
		return mapUnsubscribeError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// GetPreferences handles GET /api/v1/u/:token/preferences.
// Returns the full subscription preference list for the preference center.
func (h *UnsubscribeHandler) GetPreferences(c *echo.Context) error {
	token := c.Param("token")
	view, err := h.svc.GetPreferences(c.Request().Context(), token)
	if err != nil {
		return mapUnsubscribeError(c, err)
	}
	out := response.PreferencesViewResponse{
		WorkspaceName: view.WorkspaceName,
		Email:         view.Email,
		OptedOutOfAll: view.OptedOutOfAll,
		Entries:       make([]response.PreferencesEntryResponse, 0, len(view.Entries)),
	}
	for _, e := range view.Entries {
		out.Entries = append(out.Entries, response.PreferencesEntryResponse{
			TemplateTypeSlug: e.TemplateTypeSlug,
			TemplateTypeName: e.TemplateTypeName,
			Description:      e.Description,
			Subscribed:       e.Subscribed,
			LastReceivedAt:   e.LastReceivedAt,
		})
	}
	return c.JSON(http.StatusOK, out)
}

// UpdatePreferences handles POST /api/v1/u/:token/preferences.
// Persists a batch of subscription toggles from the preference center form.
func (h *UnsubscribeHandler) UpdatePreferences(c *echo.Context) error {
	token := c.Param("token")
	var req request.UpdatePreferencesRequest
	if err := c.Bind(&req); err != nil {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	if len(req.Changes) == 0 {
		return response.WriteError(c, http.StatusBadRequest, "BAD_REQUEST", "changes must not be empty")
	}
	changes := make([]service.PreferenceChange, 0, len(req.Changes))
	for _, ch := range req.Changes {
		changes = append(changes, service.PreferenceChange{
			TemplateTypeSlug: ch.TemplateTypeSlug,
			Subscribed:       ch.Subscribed,
		})
	}
	if err := h.svc.UpdatePreferences(c.Request().Context(), token, changes); err != nil {
		return mapUnsubscribeError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// Resubscribe handles POST /api/v1/u/:token/resubscribe.
// Re-enables email delivery for the address encoded in the token.
func (h *UnsubscribeHandler) Resubscribe(c *echo.Context) error {
	token := c.Param("token")
	if err := h.svc.Resubscribe(c.Request().Context(), token); err != nil {
		return mapUnsubscribeError(c, err)
	}
	return c.NoContent(http.StatusOK)
}

// mapUnsubscribeError converts service errors to HTTP responses.
// Internal errors are never leaked — only two states are exposed.
func mapUnsubscribeError(c *echo.Context, err error) error {
	if errors.Is(err, service.ErrInvalidToken) {
		return response.WriteError(c, http.StatusNotFound, "NOT_FOUND", "invalid or expired link")
	}
	return response.WriteError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected error")
}
