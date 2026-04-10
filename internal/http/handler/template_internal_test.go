package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
)

func TestHeadersForTemplateTestSend_UsesExternalFilteredHeadersWhenPresent(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/templates/test-send", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("X-Signature", "sig")
	req.Header.Set("Authorization", "Bearer should-not-pass")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextKeyExternalIntegrationAllowedHeaders, map[string]string{
		"x-tenant-code": "acme",
		"x-signature":   "sig",
	})

	headers := headersForTemplateTestSend(c)
	if len(headers) != 2 {
		t.Fatalf("expected exactly 2 filtered headers, got %#v", headers)
	}
	if headers["x-tenant-code"] != "acme" || headers["x-signature"] != "sig" {
		t.Fatalf("unexpected filtered headers: %#v", headers)
	}
	if _, ok := headers["Authorization"]; ok {
		t.Fatalf("authorization header must not be forwarded: %#v", headers)
	}
}

func TestHeadersForTemplateTestSend_FallsBackToFirstHeaderValues(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/templates/test-send", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Add("X-Tenant-Code", "ignored")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	headers := headersForTemplateTestSend(c)
	if headers["X-Tenant-Code"] != "acme" {
		t.Fatalf("expected fallback to first request header value, got %#v", headers)
	}
}

func TestHeadersForTemplateTestSend_DoesNotFallbackWhenExternalContextPresent(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/templates/test-send", nil)
	req.Header.Set("X-Tenant-Code", "acme")
	req.Header.Set("Authorization", "Bearer should-not-pass")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextKeyExternalIntegrationProfile, domain.ExternalIntegrationProfile{Slug: "partner-portal"})
	c.Set(middleware.ContextKeyExternalIntegrationAllowedHeaders, map[string]string{})

	headers := headersForTemplateTestSend(c)
	if len(headers) != 0 {
		t.Fatalf("expected no fallback to raw request headers, got %#v", headers)
	}
	if _, ok := headers["Authorization"]; ok {
		t.Fatalf("authorization header must not be forwarded: %#v", headers)
	}
}
