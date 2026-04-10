package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/service"
)

// --- Mocks for TrackingHandler ---

type mockTrackingEmailStore struct {
	getByTrackingIDFn func(ctx context.Context, trackingID string) (*domain.Email, error)
	callCount         atomic.Int32
}

func (m *mockTrackingEmailStore) GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error) {
	m.callCount.Add(1)
	if m.getByTrackingIDFn != nil {
		return m.getByTrackingIDFn(ctx, trackingID)
	}
	return nil, domain.ErrNotFound
}
func (m *mockTrackingEmailStore) PurgeWorkspaceRuntime(_ context.Context, _ uuid.UUID) error {
	return nil
}

// newTrackingEventProcessor builds a real EventProcessor backed by tracking mocks.
func newTrackingEventProcessor(lookup service.EmailLookup, updater service.EmailStatusUpdater) *service.EventProcessor {
	return service.NewEventProcessor(lookup, updater, &mockTrackingSuppressionWriter{}, nil, nil)
}

type mockTrackingSuppressionWriter struct{}

func (m *mockTrackingSuppressionWriter) AddGlobal(_ context.Context, _ *domain.SuppressionGlobal) error {
	return nil
}
func (m *mockTrackingSuppressionWriter) AddWorkspace(_ context.Context, _ *domain.SuppressionWorkspace) error {
	return nil
}

// mockTrackingEmailUpdater counts how many times AddEvent is called.
type mockTrackingEmailUpdater struct {
	addEventCount atomic.Int32
}

func (m *mockTrackingEmailUpdater) UpdateStatus(_ context.Context, _ uuid.UUID, _, _ domain.EmailStatus) error {
	return nil
}

func (m *mockTrackingEmailUpdater) AddEvent(_ context.Context, _ *domain.EmailEvent) error {
	m.addEventCount.Add(1)
	return nil
}

// mockTrackingLookup implements service.EmailLookup (GetByProviderMessageID) as a no-op
// — not needed for ProcessDirect, but required to satisfy the interface.
type mockTrackingLookup struct{}

func (m *mockTrackingLookup) GetByProviderMessageID(_ context.Context, _ string) (*domain.Email, error) {
	return nil, errors.New("not used in direct path")
}

// buildTrackingRequest creates an echo.Context for GET /t/o/:tracking_id.
func buildTrackingRequest(t *testing.T, trackingID string) *echo.Context {
	t.Helper()
	c, _ := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodGet, "/t/o/"+trackingID, nil),
		PathValues: echo.PathValues{
			{Name: "tracking_id", Value: trackingID},
		},
	}.ToContextRecorder(t)
	return c
}

// waitForGoroutines gives background goroutines a short window to run.
// The value is deliberately small — tests should be fast.
func waitForGoroutines() {
	time.Sleep(50 * time.Millisecond)
}

// --- C14 Tests ---

// TestTrackingHandler_DuplicateOpen_SkipsGoroutine verifies that two identical
// tracking requests within 30 s only spawn one background goroutine.
func TestTrackingHandler_DuplicateOpen_SkipsGoroutine(t *testing.T) {
	emailID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())

	emailStore := &mockTrackingEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			return &domain.Email{
				ID:          emailID,
				WorkspaceID: wsID,
				TrackingID:  trackingID,
				Status:      domain.StatusDelivered,
			}, nil
		},
	}
	updater := &mockTrackingEmailUpdater{}
	ep := newTrackingEventProcessor(&mockTrackingLookup{}, updater)
	h := handler.NewTrackingHandler(context.Background(), emailStore, ep, nil)

	// First request — goroutine should be spawned.
	c1 := buildTrackingRequest(t, "trk_abc")
	if err := h.HandleOpen(c1); err != nil {
		t.Fatalf("first HandleOpen: unexpected error: %v", err)
	}

	// Second request with the same tracking_id — goroutine should be skipped.
	c2 := buildTrackingRequest(t, "trk_abc")
	if err := h.HandleOpen(c2); err != nil {
		t.Fatalf("second HandleOpen: unexpected error: %v", err)
	}

	waitForGoroutines()

	// Pixel returned for both requests.
	// Only one goroutine should have touched the email store (and called AddEvent).
	count := updater.addEventCount.Load()
	if count != 1 {
		t.Fatalf("expected AddEvent called once (dedup), got %d", count)
	}
}

// TestTrackingHandler_DifferentIDs_BothGoroutinesSpawn verifies that two
// different tracking_ids both trigger their own goroutine.
func TestTrackingHandler_DifferentIDs_BothGoroutinesSpawn(t *testing.T) {
	emailID := uuid.Must(uuid.NewV7())
	wsID := uuid.Must(uuid.NewV7())

	emailStore := &mockTrackingEmailStore{
		getByTrackingIDFn: func(_ context.Context, trackingID string) (*domain.Email, error) {
			return &domain.Email{
				ID:          emailID,
				WorkspaceID: wsID,
				TrackingID:  trackingID,
				Status:      domain.StatusDelivered,
			}, nil
		},
	}
	updater := &mockTrackingEmailUpdater{}
	ep := newTrackingEventProcessor(&mockTrackingLookup{}, updater)
	h := handler.NewTrackingHandler(context.Background(), emailStore, ep, nil)

	c1 := buildTrackingRequest(t, "trk_aaa")
	if err := h.HandleOpen(c1); err != nil {
		t.Fatalf("first HandleOpen: unexpected error: %v", err)
	}

	c2 := buildTrackingRequest(t, "trk_bbb")
	if err := h.HandleOpen(c2); err != nil {
		t.Fatalf("second HandleOpen: unexpected error: %v", err)
	}

	waitForGoroutines()

	count := updater.addEventCount.Load()
	if count != 2 {
		t.Fatalf("expected AddEvent called twice (distinct IDs), got %d", count)
	}
}

// --- C16 Tests ---

// TestTrackingHandler_Drain_ReturnsAfterGoroutines verifies that Drain() waits
// for all in-flight goroutines and returns, even when the lifecycleCtx is cancelled.
func TestTrackingHandler_Drain_ReturnsAfterGoroutines(t *testing.T) {
	// Gate channel: keeps the goroutine inside its work until we release it.
	gate := make(chan struct{})

	emailStore := &mockTrackingEmailStore{
		getByTrackingIDFn: func(ctx context.Context, _ string) (*domain.Email, error) {
			// Block until the gate is closed or ctx is cancelled.
			select {
			case <-gate:
			case <-ctx.Done():
			}
			return nil, domain.ErrNotFound
		},
	}

	lifecycleCtx, cancel := context.WithCancel(context.Background())
	ep := newTrackingEventProcessor(&mockTrackingLookup{}, &mockTrackingEmailUpdater{})
	h := handler.NewTrackingHandler(lifecycleCtx, emailStore, ep, nil)

	// Trigger a goroutine.
	c := buildTrackingRequest(t, "trk_drain_test")
	if err := h.HandleOpen(c); err != nil {
		t.Fatalf("HandleOpen error: %v", err)
	}

	// Cancel lifecycle context — this unblocks the goroutine via ctx.Done().
	cancel()

	// Drain must return in reasonable time, not hang.
	done := make(chan struct{})
	go func() {
		h.Drain()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Drain() did not return within 2s — goroutine leak detected")
	}

	// Also close the gate in case ctx cancellation raced.
	close(gate)
}

// TestTrackingHandler_AlwaysReturnsPixel verifies the pixel is always returned,
// even when the goroutine is skipped on a duplicate.
func TestTrackingHandler_AlwaysReturnsPixel(t *testing.T) {
	emailStore := &mockTrackingEmailStore{
		getByTrackingIDFn: func(_ context.Context, _ string) (*domain.Email, error) {
			return nil, domain.ErrNotFound
		},
	}
	ep := newTrackingEventProcessor(&mockTrackingLookup{}, &mockTrackingEmailUpdater{})
	h := handler.NewTrackingHandler(context.Background(), emailStore, ep, nil)

	for i := range 3 {
		rec := httptest.NewRecorder()
		c, _ := echotest.ContextConfig{
			Request: httptest.NewRequest(http.MethodGet, "/t/o/trk_pixel", nil),
			PathValues: echo.PathValues{
				{Name: "tracking_id", Value: "trk_pixel"},
			},
			Response: rec,
		}.ToContextRecorder(t)
		if err := h.HandleOpen(c); err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 OK, got %d", i, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/gif" {
			t.Fatalf("request %d: expected Content-Type image/gif, got %q", i, ct)
		}
	}
}
