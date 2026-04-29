package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/service"
)

// --- Fake unsubscribeService ---

type fakeUnsubscribeService struct {
	getContextFn        func(ctx context.Context, token string) (*service.UnsubscribeContext, error)
	oneClickOptOutFn    func(ctx context.Context, token string) error
	optOutAllFn         func(ctx context.Context, token string) error
	getPreferencesFn    func(ctx context.Context, token string) (*service.PreferencesView, error)
	updatePreferencesFn func(ctx context.Context, token string, changes []service.PreferenceChange) error
	resubscribeFn       func(ctx context.Context, token string) error
}

func (f *fakeUnsubscribeService) GetContext(ctx context.Context, token string) (*service.UnsubscribeContext, error) {
	if f.getContextFn != nil {
		return f.getContextFn(ctx, token)
	}
	return nil, nil
}

func (f *fakeUnsubscribeService) OneClickOptOut(ctx context.Context, token string) error {
	if f.oneClickOptOutFn != nil {
		return f.oneClickOptOutFn(ctx, token)
	}
	return nil
}

func (f *fakeUnsubscribeService) OptOutAll(ctx context.Context, token string) error {
	if f.optOutAllFn != nil {
		return f.optOutAllFn(ctx, token)
	}
	return nil
}

func (f *fakeUnsubscribeService) GetPreferences(ctx context.Context, token string) (*service.PreferencesView, error) {
	if f.getPreferencesFn != nil {
		return f.getPreferencesFn(ctx, token)
	}
	return nil, nil
}

func (f *fakeUnsubscribeService) UpdatePreferences(ctx context.Context, token string, changes []service.PreferenceChange) error {
	if f.updatePreferencesFn != nil {
		return f.updatePreferencesFn(ctx, token, changes)
	}
	return nil
}

func (f *fakeUnsubscribeService) Resubscribe(ctx context.Context, token string) error {
	if f.resubscribeFn != nil {
		return f.resubscribeFn(ctx, token)
	}
	return nil
}

// --- Setup helper ---

func setupUnsubTest(svc *fakeUnsubscribeService) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	e.Use(middleware.Scope())

	h := handler.NewUnsubscribeHandler(svc)

	e.GET("/api/v1/u/:token", h.GetContext)
	e.POST("/api/v1/u/:token", h.OneClickOptOut)
	e.POST("/api/v1/u/:token/all", h.OptOutAll)
	e.POST("/api/v1/u/:token/resubscribe", h.Resubscribe)
	e.GET("/api/v1/u/:token/preferences", h.GetPreferences)
	e.POST("/api/v1/u/:token/preferences", h.UpdatePreferences)

	return e
}

// --- Tests ---

func TestUnsubHandler_GetContext_Success(t *testing.T) {
	desc := "Newsletter emails"
	svc := &fakeUnsubscribeService{
		getContextFn: func(_ context.Context, token string) (*service.UnsubscribeContext, error) {
			if token != "validtoken" {
				t.Errorf("unexpected token: %s", token)
			}
			return &service.UnsubscribeContext{
				WorkspaceName:    "Acme",
				TemplateTypeSlug: "newsletter",
				TemplateTypeName: "Newsletter",
				Email:            "user@example.com",
				OptedOutOfType:   false,
				OptedOutOfAll:    false,
			}, nil
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/u/validtoken", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["workspace_name"] != "Acme" {
		t.Errorf("expected workspace_name 'Acme', got %v", body["workspace_name"])
	}
	if body["template_type_slug"] != "newsletter" {
		t.Errorf("expected template_type_slug 'newsletter', got %v", body["template_type_slug"])
	}
	if body["email"] != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %v", body["email"])
	}
	if body["opted_out_of_type"] != false {
		t.Errorf("expected opted_out_of_type false, got %v", body["opted_out_of_type"])
	}
	if body["opted_out_of_all"] != false {
		t.Errorf("expected opted_out_of_all false, got %v", body["opted_out_of_all"])
	}
	_ = desc // suppress unused warning; Description is on PreferencesEntry, not UnsubscribeContext
}

func TestUnsubHandler_GetContext_InvalidToken_404(t *testing.T) {
	svc := &fakeUnsubscribeService{
		getContextFn: func(_ context.Context, _ string) (*service.UnsubscribeContext, error) {
			return nil, service.ErrInvalidToken
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/u/garbage", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "NOT_FOUND")
}

func TestUnsubHandler_OneClickOptOut_Success(t *testing.T) {
	var calledWith string
	svc := &fakeUnsubscribeService{
		oneClickOptOutFn: func(_ context.Context, token string) error {
			calledWith = token
			return nil
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/tok123", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if calledWith != "tok123" {
		t.Errorf("expected service called with 'tok123', got %q", calledWith)
	}
}

func TestUnsubHandler_OneClickOptOut_InvalidToken_404(t *testing.T) {
	svc := &fakeUnsubscribeService{
		oneClickOptOutFn: func(_ context.Context, _ string) error {
			return fmt.Errorf("wrapped: %w", service.ErrInvalidToken)
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/garbage", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "NOT_FOUND")
}

func TestUnsubHandler_OptOutAll_Success(t *testing.T) {
	var calledWith string
	svc := &fakeUnsubscribeService{
		optOutAllFn: func(_ context.Context, token string) error {
			calledWith = token
			return nil
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/tok456/all", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if calledWith != "tok456" {
		t.Errorf("expected service called with 'tok456', got %q", calledWith)
	}
}

func TestUnsubHandler_GetPreferences_Success(t *testing.T) {
	desc := "Weekly updates"
	svc := &fakeUnsubscribeService{
		getPreferencesFn: func(_ context.Context, _ string) (*service.PreferencesView, error) {
			return &service.PreferencesView{
				WorkspaceName: "Acme",
				Email:         "user@example.com",
				OptedOutOfAll: false,
				Entries: []service.PreferencesEntry{
					{
						TemplateTypeSlug: "newsletter",
						TemplateTypeName: "Newsletter",
						Description:      &desc,
						Subscribed:       true,
						LastReceivedAt:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
					},
					{
						TemplateTypeSlug: "promo",
						TemplateTypeName: "Promotions",
						Description:      nil,
						Subscribed:       false,
						LastReceivedAt:   time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC),
					},
				},
			}, nil
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/u/tok789/preferences", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["workspace_name"] != "Acme" {
		t.Errorf("expected workspace_name 'Acme', got %v", body["workspace_name"])
	}
	entries, ok := body["entries"].([]any)
	if !ok {
		t.Fatalf("expected entries array, got %T", body["entries"])
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	first := entries[0].(map[string]any)
	if first["template_type_slug"] != "newsletter" {
		t.Errorf("expected first slug 'newsletter', got %v", first["template_type_slug"])
	}
	if first["description"] != "Weekly updates" {
		t.Errorf("expected description 'Weekly updates', got %v", first["description"])
	}
	second := entries[1].(map[string]any)
	if _, hasDesc := second["description"]; hasDesc {
		t.Errorf("expected no description field on second entry (nil), got %v", second["description"])
	}
}

func TestUnsubHandler_UpdatePreferences_PersistsChanges(t *testing.T) {
	var receivedToken string
	var receivedChanges []service.PreferenceChange

	svc := &fakeUnsubscribeService{
		updatePreferencesFn: func(_ context.Context, token string, changes []service.PreferenceChange) error {
			receivedToken = token
			receivedChanges = changes
			return nil
		},
	}

	e := setupUnsubTest(svc)
	body := `{"changes":[{"template_type_slug":"newsletter","subscribed":false},{"template_type_slug":"promo","subscribed":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/mytoken/preferences", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if receivedToken != "mytoken" {
		t.Errorf("expected token 'mytoken', got %q", receivedToken)
	}
	if len(receivedChanges) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(receivedChanges))
	}
	if receivedChanges[0].TemplateTypeSlug != "newsletter" || receivedChanges[0].Subscribed != false {
		t.Errorf("unexpected first change: %+v", receivedChanges[0])
	}
	if receivedChanges[1].TemplateTypeSlug != "promo" || receivedChanges[1].Subscribed != true {
		t.Errorf("unexpected second change: %+v", receivedChanges[1])
	}
}

func TestUnsubHandler_UpdatePreferences_BadRequest(t *testing.T) {
	svc := &fakeUnsubscribeService{}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/tok/preferences", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec, "BAD_REQUEST")
}

func TestUnsubHandler_Resubscribe_Success(t *testing.T) {
	var calledWith string
	svc := &fakeUnsubscribeService{
		resubscribeFn: func(_ context.Context, token string) error {
			calledWith = token
			return nil
		},
	}

	e := setupUnsubTest(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/u/resubtoken/resubscribe", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if calledWith != "resubtoken" {
		t.Errorf("expected service called with 'resubtoken', got %q", calledWith)
	}
}

// --- Test helpers ---

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'error' object in response, got %T", body["error"])
	}
	if errObj["code"] != expectedCode {
		t.Errorf("expected error code %q, got %v", expectedCode, errObj["code"])
	}
}
