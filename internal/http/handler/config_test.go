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
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
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

	oidc := handler.OIDCInfo{
		DiscoveryURL:    "https://sso.example.com/realms/senda/.well-known/openid-configuration",
		ClientID:        "senda-web",
		ClientSecretSet: true,
	}
	h := handler.NewConfigHandler(cs, oidc)
	e.GET("/api/v1/manage/config", h.Get)
	e.PUT("/api/v1/manage/config", h.Update)
	return e
}

// --- Tests ---

func TestConfigHandler_Get_Success(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:              3,
		RetryBackoffBaseSeconds:        60,
		LogRetentionDays:               90,
		BounceAlertThresholdPercent:    5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:     24,
		OnboardingCompleted:            true,
		ExternalIntegrations: []domain.ExternalIntegrationProfile{
			{
				Slug:           "partner-portal",
				Name:           "Partner Portal",
				Description:    "Integration for partner-facing UI",
				Enabled:        true,
				AuthMethodName: "signed-headers",
				ResolverName:   "tenant-workspace-resolver",
			},
		},
		UpdatedAt: now,
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

	// Decode into nested response matching frontend SystemSettings contract.
	var resp response.ConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// OIDC section (read-only from env).
	if resp.OIDC.DiscoveryURL != "https://sso.example.com/realms/senda/.well-known/openid-configuration" {
		t.Fatalf("expected oidc.discovery_url, got %q", resp.OIDC.DiscoveryURL)
	}
	if resp.OIDC.ClientID != "senda-web" {
		t.Fatalf("expected oidc.client_id 'senda-web', got %q", resp.OIDC.ClientID)
	}
	if !resp.OIDC.ClientSecretSet {
		t.Fatal("expected oidc.client_secret_set=true")
	}

	// Email defaults section.
	if resp.EmailDefaults.MaxRetries != 3 {
		t.Fatalf("expected email_defaults.max_retries 3, got %d", resp.EmailDefaults.MaxRetries)
	}
	if resp.EmailDefaults.BackoffBaseSeconds != 60 {
		t.Fatalf("expected email_defaults.backoff_base_seconds 60, got %d", resp.EmailDefaults.BackoffBaseSeconds)
	}
	if resp.EmailDefaults.LogRetentionDays != 90 {
		t.Fatalf("expected email_defaults.log_retention_days 90, got %d", resp.EmailDefaults.LogRetentionDays)
	}

	// Alerts section.
	if resp.Alerts.BounceThresholdPercent != 5.0 {
		t.Fatalf("expected alerts.bounce_threshold_percent 5.0, got %f", resp.Alerts.BounceThresholdPercent)
	}
	if resp.Alerts.ComplaintThresholdPercent != 0.1 {
		t.Fatalf("expected alerts.complaint_threshold_percent 0.1, got %f", resp.Alerts.ComplaintThresholdPercent)
	}

	// Domain section.
	if resp.Domain.RecheckIntervalHours != 24 {
		t.Fatalf("expected domain.recheck_interval_hours 24, got %d", resp.Domain.RecheckIntervalHours)
	}
	if len(resp.ExternalIntegrations.Profiles) != 1 {
		t.Fatalf("expected 1 external integration profile, got %d", len(resp.ExternalIntegrations.Profiles))
	}
	if resp.ExternalIntegrations.Profiles[0].Slug != "partner-portal" {
		t.Fatalf("expected external integration slug partner-portal, got %q", resp.ExternalIntegrations.Profiles[0].Slug)
	}
}

func TestConfigHandler_Update_Success(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:              3,
		RetryBackoffBaseSeconds:        60,
		LogRetentionDays:               90,
		BounceAlertThresholdPercent:    5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:     24,
		OnboardingCompleted:            false,
		UpdatedAt:                      now,
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

	// Nested request matching frontend UpdateSettingsRequest.
	body := `{"email_defaults":{"max_retries":5},"alerts":{"bounce_threshold_percent":10.0}}`
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
	if upserted.BounceAlertThresholdPercent != 10.0 {
		t.Fatalf("expected bounce_alert_threshold_percent 10.0, got %f", upserted.BounceAlertThresholdPercent)
	}
	// Fields not in request should remain unchanged.
	if upserted.LogRetentionDays != 90 {
		t.Fatalf("expected log_retention_days 90 (unchanged), got %d", upserted.LogRetentionDays)
	}
	if upserted.DomainRecheckIntervalHours != 24 {
		t.Fatalf("expected domain_recheck_interval_hours 24 (unchanged), got %d", upserted.DomainRecheckIntervalHours)
	}
}

func TestConfigHandler_Update_PartialFields(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:              3,
		RetryBackoffBaseSeconds:        60,
		LogRetentionDays:               90,
		BounceAlertThresholdPercent:    5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:     24,
		OnboardingCompleted:            false,
		UpdatedAt:                      now,
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

	// Only update domain section.
	body := `{"domain":{"recheck_interval_hours":48}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if upserted.DomainRecheckIntervalHours != 48 {
		t.Fatalf("expected domain_recheck_interval_hours 48, got %d", upserted.DomainRecheckIntervalHours)
	}
	// Other fields unchanged.
	if upserted.DefaultRetryCount != 3 {
		t.Fatalf("expected default_retry_count 3, got %d", upserted.DefaultRetryCount)
	}
	if upserted.RetryBackoffBaseSeconds != 60 {
		t.Fatalf("expected retry_backoff_base_seconds 60, got %d", upserted.RetryBackoffBaseSeconds)
	}
	if upserted.LogRetentionDays != 90 {
		t.Fatalf("expected log_retention_days 90, got %d", upserted.LogRetentionDays)
	}
}

func TestConfigHandler_Update_ExternalIntegrations(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:              3,
		RetryBackoffBaseSeconds:        60,
		LogRetentionDays:               90,
		BounceAlertThresholdPercent:    5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:     24,
		OnboardingCompleted:            false,
		UpdatedAt:                      now,
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

	body := `{
		"external_integrations":{
			"profiles":[
				{
					"slug":"partner-portal",
					"name":"Partner Portal",
					"description":"Integration for partner-facing UI",
					"enabled":true,
					"auth_method_name":"signed-headers",
					"resolver_name":"tenant-workspace-resolver",
					"allowed_headers":["X-Tenant-Code","X-Trace-ID"],
					"required_headers":["x-tenant-code"]
				}
			]
		}
	}`
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
	if len(upserted.ExternalIntegrations) != 1 {
		t.Fatalf("expected 1 external integration profile, got %d", len(upserted.ExternalIntegrations))
	}
	profile := upserted.ExternalIntegrations[0]
	if profile.Slug != "partner-portal" {
		t.Fatalf("expected normalized slug partner-portal, got %q", profile.Slug)
	}
	if len(profile.RequiredHeaders) != 1 || profile.RequiredHeaders[0] != "x-tenant-code" {
		t.Fatalf("expected normalized required header x-tenant-code, got %#v", profile.RequiredHeaders)
	}
}

func TestConfigHandler_Update_ExternalIntegrationsValidation(t *testing.T) {
	now := time.Now().UTC()
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:              3,
		RetryBackoffBaseSeconds:        60,
		LogRetentionDays:               90,
		BounceAlertThresholdPercent:    5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:     24,
		OnboardingCompleted:            false,
		UpdatedAt:                      now,
	}

	upsertCalled := false
	cs := &mockGlobalConfigStore{
		getFn: func(_ context.Context) (*domain.GlobalConfig, error) {
			return cfg, nil
		},
		upsertFn: func(_ context.Context, c *domain.GlobalConfig) error {
			upsertCalled = true
			return nil
		},
	}

	e := setupConfigTest(cs)

	body := `{"external_integrations":{"profiles":[{"slug":"partner-portal","name":"Partner Portal","description":"Integration for partner-facing UI","enabled":true,"auth_method_name":"signed-headers","resolver_name":"tenant-workspace-resolver","allowed_headers":["x-trace-id"],"required_headers":["x-tenant-code"]}]}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/manage/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 validation error, got %d: %s", rec.Code, rec.Body.String())
	}
	if upsertCalled {
		t.Fatal("expected invalid config to be rejected before upsert")
	}
}
