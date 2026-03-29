package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/internal/http/handler"
)

type mockPingerOK struct{}

func (m *mockPingerOK) Ping(_ context.Context) error { return nil }

type mockPingerFail struct{}

func (m *mockPingerFail) Ping(_ context.Context) error { return errors.New("connection refused") }

func TestHealthHandler_HealthyWithDB(t *testing.T) {
	h := handler.NewHealthHandler(&mockPingerOK{})
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/healthz", nil),
	}.ToContextRecorder(t)

	if err := h.Health(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected status 'healthy', got %q", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected 'checks' object in response")
	}
	if checks["database"] != "ok" {
		t.Fatalf("expected database check 'ok', got %q", checks["database"])
	}
}

func TestHealthHandler_HealthyWithoutDB(t *testing.T) {
	h := handler.NewHealthHandler(nil)
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/healthz", nil),
	}.ToContextRecorder(t)

	if err := h.Health(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("expected status 'healthy', got %q", body["status"])
	}
	// No checks field expected when pinger is nil.
	if _, ok := body["checks"]; ok {
		t.Fatal("expected no 'checks' field when pinger is nil")
	}
}

func TestHealthHandler_UnhealthyDB(t *testing.T) {
	h := handler.NewHealthHandler(&mockPingerFail{})
	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/healthz", nil),
	}.ToContextRecorder(t)

	if err := h.Health(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected status 'unhealthy', got %q", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("expected 'checks' object in response")
	}
	if checks["database"] != "connection refused" {
		t.Fatalf("expected database error message, got %q", checks["database"])
	}
}
