package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
)

// --- Helpers ---

// fakeWorkspaceContext injects a workspace_id into the echo context (simulating API key auth).
func fakeWorkspaceContext(wsID uuid.UUID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set("workspace_id", wsID)
			return next(c)
		}
	}
}

func setupSendTest() (*echo.Echo, *handler.SendHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	// Inject a workspace context to simulate API key auth.
	e.Use(fakeWorkspaceContext(uuid.Must(uuid.NewV7())))

	// nil SendService is safe for validation-only tests (handler validates before calling service).
	h := handler.NewSendHandler(nil)
	e.POST("/api/v1/send", h.Send)

	return e, h
}

// setupSendTestNoWorkspace sets up a send test WITHOUT workspace context.
func setupSendTestNoWorkspace() (*echo.Echo, *handler.SendHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewSendHandler(nil)
	e.POST("/api/v1/send", h.Send)

	return e, h
}

// --- Tests ---

func TestSendHandler_Send_MissingWorkspaceContext(t *testing.T) {
	e, _ := setupSendTestNoWorkspace()

	body := `{"ref":"acme:default:welcome","to":["user@example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_Send_MissingRef(t *testing.T) {
	e, _ := setupSendTest()

	body := `{"to":["user@example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if errResp.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %s", errResp.Error.Code)
	}
	if len(errResp.Error.Details) != 1 || errResp.Error.Details[0].Field != "ref" {
		t.Fatalf("expected field error on 'ref', got %+v", errResp.Error.Details)
	}
}

func TestSendHandler_Send_MissingTo(t *testing.T) {
	e, _ := setupSendTest()

	body := `{"ref":"acme:default:welcome"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(errResp.Error.Details) != 1 || errResp.Error.Details[0].Field != "to" {
		t.Fatalf("expected field error on 'to', got %+v", errResp.Error.Details)
	}
}

func TestSendHandler_Send_InvalidBody(t *testing.T) {
	e, _ := setupSendTest()

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_Send_MissingBothRefAndTo(t *testing.T) {
	e, _ := setupSendTest()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(errResp.Error.Details) != 2 {
		t.Fatalf("expected 2 field errors, got %d: %+v", len(errResp.Error.Details), errResp.Error.Details)
	}
}

func TestSendHandler_Send_EmptyTo(t *testing.T) {
	e, _ := setupSendTest()

	body := `{"ref":"acme:default:welcome","to":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_Send_TooManyRecipients(t *testing.T) {
	e, _ := setupSendTest()

	recipients := make([]string, 0, 51)
	for i := 0; i < 51; i++ {
		recipients = append(recipients, `"user`+string(rune('a'+(i%26)))+`@example.com"`)
	}
	body := `{"ref":"acme:default:welcome","to":[` + strings.Join(recipients, ",") + `]}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(errResp.Error.Details) != 1 || errResp.Error.Details[0].Field != "to" {
		t.Fatalf("expected field error on 'to', got %+v", errResp.Error.Details)
	}
}
