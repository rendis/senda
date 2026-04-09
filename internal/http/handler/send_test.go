package handler_test

import (
	"context"
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
	"github.com/rendis/senda/internal/service"
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
	h := handler.NewSendHandler(nil, 100)
	e.POST("/api/v1/send", h.Send)
	e.POST("/api/v1/send/batch", h.SendBatch)

	return e, h
}

// setupSendTestNoWorkspace sets up a send test WITHOUT workspace context.
func setupSendTestNoWorkspace() (*echo.Echo, *handler.SendHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewSendHandler(nil, 100)
	e.POST("/api/v1/send", h.Send)
	e.POST("/api/v1/send/batch", h.SendBatch)

	return e, h
}

type fakeSendService struct {
	sendFn      func(*service.SendRequest) *service.SendResponse
	sendBatchFn func(*service.SendBatchRequest) *service.SendBatchResponse
}

func (f *fakeSendService) Send(_ context.Context, req *service.SendRequest) (*service.SendResponse, error) {
	if f.sendFn != nil {
		return f.sendFn(req), nil
	}
	return &service.SendResponse{}, nil
}

func (f *fakeSendService) SendBatch(_ context.Context, req *service.SendBatchRequest) (*service.SendBatchResponse, error) {
	if f.sendBatchFn != nil {
		return f.sendBatchFn(req), nil
	}
	return &service.SendBatchResponse{}, nil
}

func setupSendBatchTest(svc *fakeSendService, maxItems int) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(fakeWorkspaceContext(uuid.Must(uuid.NewV7())))

	h := handler.NewSendHandler(svc, maxItems)
	e.POST("/api/v1/send/batch", h.SendBatch)

	return e
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

func TestSendHandler_SendBatch_MissingRef(t *testing.T) {
	e := setupSendBatchTest(nil, 100)

	body := `{"items":[{"to":"user@example.com"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_SendBatch_EmptyItems(t *testing.T) {
	e := setupSendBatchTest(nil, 100)

	body := `{"ref":"acme:default:welcome","items":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_SendBatch_TooManyItems(t *testing.T) {
	e := setupSendBatchTest(nil, 2)

	body := `{"ref":"acme:default:welcome","items":[{"to":"one@example.com"},{"to":"two@example.com"},{"to":"three@example.com"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_SendBatch_ItemMissingTo(t *testing.T) {
	e := setupSendBatchTest(nil, 100)

	body := `{"ref":"acme:default:welcome","items":[{}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_SendBatch_ValidRequest(t *testing.T) {
	svc := &fakeSendService{
		sendBatchFn: func(req *service.SendBatchRequest) *service.SendBatchResponse {
			if req.Ref != "acme:default:welcome" {
				t.Fatalf("unexpected ref %q", req.Ref)
			}
			if len(req.Items) != 1 || req.Items[0].To != "user@example.com" {
				t.Fatalf("unexpected items %+v", req.Items)
			}
			return &service.SendBatchResponse{
				Status:           "accepted",
				TemplateResolved: req.Ref,
				AcceptedCount:    1,
				Items: []service.SendBatchItemResult{
					{
						Index:      0,
						To:         "user@example.com",
						TrackingID: "trk_batch_1",
						Status:     "accepted",
					},
				},
			}
		},
	}
	e := setupSendBatchTest(svc, 100)

	body := `{"ref":"acme:default:welcome","items":[{"to":"user@example.com","variables":{"name":"Jane"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSendHandler_Send_PassesInjectorsFromRequestBody(t *testing.T) {
	svc := &fakeSendService{
		sendFn: func(req *service.SendRequest) *service.SendResponse {
			if req.Injectors["student"]["name"] != "Jane Doe" {
				t.Fatalf("unexpected injectors payload: %+v", req.Injectors)
			}
			return &service.SendResponse{Status: "accepted"}
		},
	}

	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(fakeWorkspaceContext(uuid.Must(uuid.NewV7())))
	h := handler.NewSendHandler(svc, 100)
	e.POST("/api/v1/send", h.Send)

	body := `{"ref":"acme:default:welcome","to":["user@example.com"],"injectors":{"student":{"name":"Jane Doe"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}
