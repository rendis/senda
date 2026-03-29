package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
)

func setupSendErrorContext(t *testing.T) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	return c, rec
}

func TestMapSendError_TemplateDisabled(t *testing.T) {
	c, rec := setupSendErrorContext(t)

	if err := mapSendError(c, domain.ErrTemplateDisabled); err != nil {
		t.Fatalf("mapSendError returned unexpected error: %v", err)
	}

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}

	var payload response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "TEMPLATE_DISABLED" {
		t.Fatalf("expected TEMPLATE_DISABLED, got %s", payload.Error.Code)
	}
}

func TestMapSendError_WorkspaceScopeMismatch(t *testing.T) {
	c, rec := setupSendErrorContext(t)

	if err := mapSendError(c, domain.ErrWorkspaceScopeMismatch); err != nil {
		t.Fatalf("mapSendError returned unexpected error: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestMapSendError_SystemWorkspaceBlocked(t *testing.T) {
	c, rec := setupSendErrorContext(t)

	if err := mapSendError(c, domain.ErrSystemWorkspaceBlocked); err != nil {
		t.Fatalf("mapSendError returned unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestMapSendError_NoDefaultIdentity(t *testing.T) {
	c, rec := setupSendErrorContext(t)

	if err := mapSendError(c, domain.ErrNoDefaultIdentity); err != nil {
		t.Fatalf("mapSendError returned unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var payload response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "NO_DEFAULT_IDENTITY" {
		t.Fatalf("expected NO_DEFAULT_IDENTITY, got %s", payload.Error.Code)
	}
}
