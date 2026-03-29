package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/rendis/senda/internal/http/response"
	"github.com/rendis/senda/internal/port"
)

func setupDataPlaneEmailTest(es port.EmailStore, wsID uuid.UUID, withWorkspace bool) (*echo.Echo, *handler.DataPlaneEmailHandler) {
	e := echo.New()
	e.HTTPErrorHandler = response.HTTPErrorHandler
	e.Use(middleware.RequestID())
	if withWorkspace {
		e.Use(fakeWorkspaceContext(wsID))
	}

	h := handler.NewDataPlaneEmailHandler(es)
	e.GET("/api/v1/emails", h.List)
	e.GET("/api/v1/emails/export", h.Export)
	e.GET("/api/v1/emails/:tracking_id", h.GetByTrackingID)
	e.GET("/api/v1/emails/:tracking_id/events", h.GetEvents)

	return e, h
}

func TestDataPlaneEmailHandler_List_ScopedByWorkspace(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	var gotWSID uuid.UUID
	var gotFilters port.EmailFilters
	es := &mockEmailStore{
		queryByWorkspaceFn: func(_ context.Context, reqWSID uuid.UUID, filters port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
			gotWSID = reqWSID
			gotFilters = filters
			return []*domain.Email{
				{
					ID:               uuid.Must(uuid.NewV7()),
					TrackingID:       "trk_abc123",
					WorkspaceID:      wsID,
					TenantID:         uuid.Must(uuid.NewV7()),
					TemplateTypeSlug: "welcome",
					TemplateRef:      "latam:acme:welcome",
					RecipientEmail:   "user@example.com",
					FromEmail:        "no-reply@example.com",
					FromName:         "Acme",
					SubjectRendered:  "Welcome!",
					Status:           domain.StatusSent,
					AdapterID:        uuid.Must(uuid.NewV7()),
					CreatedAt:        now,
					UpdatedAt:        now,
				},
			}, "", nil
		},
	}

	e, _ := setupDataPlaneEmailTest(es, wsID, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/emails?external_id=ext-1&recipient=user@example.com&status=sent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotWSID != wsID {
		t.Fatalf("expected workspace %s, got %s", wsID, gotWSID)
	}
	if gotFilters.ExternalID == nil || *gotFilters.ExternalID != "ext-1" {
		t.Fatalf("expected external_id filter ext-1, got %+v", gotFilters.ExternalID)
	}
	if gotFilters.Recipient == nil || *gotFilters.Recipient != "user@example.com" {
		t.Fatalf("expected recipient filter user@example.com, got %+v", gotFilters.Recipient)
	}
	if gotFilters.Status == nil || *gotFilters.Status != domain.StatusSent {
		t.Fatalf("expected status filter sent, got %+v", gotFilters.Status)
	}
}

func TestDataPlaneEmailHandler_List_MissingWorkspaceContext(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	e, _ := setupDataPlaneEmailTest(&mockEmailStore{}, wsID, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/emails", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDataPlaneEmailHandler_GetByTrackingID_WrongWorkspace(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	otherWS := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return &domain.Email{
				ID:          uuid.Must(uuid.NewV7()),
				TrackingID:  "trk_wrong_ws",
				WorkspaceID: otherWS,
				TenantID:    uuid.Must(uuid.NewV7()),
				Status:      domain.StatusSent,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	e, _ := setupDataPlaneEmailTest(es, wsID, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emails/trk_wrong_ws", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDataPlaneEmailHandler_GetEvents_WrongWorkspace(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	otherWS := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()

	es := &mockEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return &domain.Email{
				ID:          uuid.Must(uuid.NewV7()),
				TrackingID:  "trk_wrong_ws",
				WorkspaceID: otherWS,
				TenantID:    uuid.Must(uuid.NewV7()),
				Status:      domain.StatusSent,
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}

	e, _ := setupDataPlaneEmailTest(es, wsID, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/emails/trk_wrong_ws/events", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDataPlaneEmailHandler_Export_CSV(t *testing.T) {
	wsID := uuid.Must(uuid.NewV7())
	now := time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)

	var gotWSID uuid.UUID
	es := &mockEmailStore{
		queryByWorkspaceFn: func(_ context.Context, reqWSID uuid.UUID, _ port.EmailFilters, _ string, _ int) ([]*domain.Email, string, error) {
			gotWSID = reqWSID
			return []*domain.Email{
				{
					ID:              uuid.Must(uuid.NewV7()),
					TrackingID:      "trk_export_1",
					WorkspaceID:     wsID,
					TenantID:        uuid.Must(uuid.NewV7()),
					TemplateRef:     "latam:acme:welcome",
					RecipientEmail:  "user@example.com",
					FromEmail:       "hello@example.com",
					SubjectRendered: "Welcome",
					Status:          domain.StatusSent,
					AdapterID:       uuid.Must(uuid.NewV7()),
					CreatedAt:       now,
					UpdatedAt:       now,
				},
			}, "", nil
		},
	}

	e, _ := setupDataPlaneEmailTest(es, wsID, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/emails/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotWSID != wsID {
		t.Fatalf("expected workspace %s, got %s", wsID, gotWSID)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("expected text/csv content type, got %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "tracking_id,external_id,recipient_email") {
		t.Fatalf("expected csv header, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "trk_export_1") {
		t.Fatalf("expected csv row with tracking id, got %s", rec.Body.String())
	}
}
