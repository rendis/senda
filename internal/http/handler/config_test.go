package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/domain"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/http/middleware"
	"github.com/senda-app/senda/internal/http/response"
	"github.com/senda-app/senda/internal/port"
)

// --- Mock GlobalConfigStore ---

type mockGlobalConfigStore struct {
	getFn    func(ctx context.Context) (*domain.GlobalConfig, error)
	upsertFn func(ctx context.Context, cfg *domain.GlobalConfig) error
}

func (m *mockGlobalConfigStore) Get(ctx context.Context) (*domain.GlobalConfig, error) {
	if m.getFn != nil {
		return m.getFn(ctx)
	}
	return nil, nil
}
func (m *mockGlobalConfigStore) Upsert(ctx context.Context, cfg *domain.GlobalConfig) error {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, cfg)
	}
	return nil
}

func setupConfigTest(cs port.GlobalConfigStore) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())

	h := handler.NewConfigHandler(cs)
	e.GET("/api/v1/manage/config", h.Get)
	e.PUT("/api/v1/manage/config", h.Update)
	return e
}

// --- Tests ---

func TestConfigHandler_Get_Success(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:             3,
		RetryBackoffBaseSeconds:       60,
		LogRetentionDays:              90,
		BounceAlertThresholdPercent:   5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:    24,
		OnboardingCompleted:           true,
		UpdatedAt:                     now,
	}

	cs := &mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) {
			return cfg, nil
		},
	}

	e := setupConfigTest(cs)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/manage/config", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.ConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.DefaultRetryCount != 3 {
		t.Fatalf("expected default_retry_count 3, got %d", resp.DefaultRetryCount)
	}
	if resp.LogRetentionDays != 90 {
		t.Fatalf("expected log_retention_days 90, got %d", resp.LogRetentionDays)
	}
	if !resp.OnboardingCompleted {
		t.Fatal("expected onboarding_completed=true")
	}
}

func TestConfigHandler_Update_Success(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:             3,
		RetryBackoffBaseSeconds:       60,
		LogRetentionDays:              90,
		BounceAlertThresholdPercent:   5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:    24,
		OnboardingCompleted:           false,
		UpdatedAt:                     now,
	}

	var upserted *domain.GlobalConfig
	cs := &mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) {
			return cfg, nil
		},
		upsertFn: func(_ context.Context, c *domain.GlobalConfig) error {
			upserted = c
			return nil
		},
	}

	e := setupConfigTest(cs)

	body := `{"default_retry_count":5,"onboarding_completed":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if upserted == nil {
		t.Fatal("expected config to be upserted")
	}
	if upserted.DefaultRetryCount != 5 {
		t.Fatalf("expected default_retry_count 5, got %d", upserted.DefaultRetryCount)
	}
	if !upserted.OnboardingCompleted {
		t.Fatal("expected onboarding_completed=true")
	}
	// Fields not in request should remain unchanged.
	if upserted.LogRetentionDays != 90 {
		t.Fatalf("expected log_retention_days 90 (unchanged), got %d", upserted.LogRetentionDays)
	}
}

func TestConfigHandler_Update_PartialFields(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:             3,
		RetryBackoffBaseSeconds:       60,
		LogRetentionDays:              90,
		BounceAlertThresholdPercent:   5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:    24,
		OnboardingCompleted:           false,
		UpdatedAt:                     now,
	}

	var upserted *domain.GlobalConfig
	cs := &mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) {
			return cfg, nil
		},
		upsertFn: func(_ context.Context, c *domain.GlobalConfig) error {
			upserted = c
			return nil
		},
	}

	e := setupConfigTest(cs)

	// Only update log_retention_days.
	body := `{"log_retention_days":180}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if upserted.LogRetentionDays != 180 {
		t.Fatalf("expected log_retention_days 180, got %d", upserted.LogRetentionDays)
	}
	// Other fields unchanged.
	if upserted.DefaultRetryCount != 3 {
		t.Fatalf("expected default_retry_count 3, got %d", upserted.DefaultRetryCount)
	}
	if upserted.RetryBackoffBaseSeconds != 60 {
		t.Fatalf("expected retry_backoff_base_seconds 60, got %d", upserted.RetryBackoffBaseSeconds)
	}
}
