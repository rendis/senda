package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/senda-app/senda/internal/http/middleware"
)

func TestOIDCOnly_OIDCPassesThrough(t *testing.T) {
	e := echo.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(middleware.ContextKeyAuthType, "oidc")
			return next(c)
		}
	})
	e.Use(middleware.OIDCOnly())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestOIDCOnly_APIKeyGetsForbidden(t *testing.T) {
	e := echo.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(middleware.ContextKeyAuthType, "apikey")
			return next(c)
		}
	})
	e.Use(middleware.OIDCOnly())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestOIDCOnly_NoAuthTypeGetsForbidden(t *testing.T) {
	e := echo.New()

	e.Use(middleware.OIDCOnly())
	e.GET("/test", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
