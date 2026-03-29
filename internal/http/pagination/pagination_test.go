package pagination_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/pagination"
)

func TestEncodeDecode_Roundtrip(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)
	id := uuid.New()

	encoded := pagination.EncodeCursor(now, id)
	if encoded == "" {
		t.Fatal("expected non-empty encoded cursor")
	}

	decoded, err := pagination.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error decoding cursor: %v", err)
	}

	if !decoded.Timestamp.Equal(now) {
		t.Fatalf("expected timestamp %v, got %v", now, decoded.Timestamp)
	}
	if decoded.ID != id {
		t.Fatalf("expected ID %s, got %s", id, decoded.ID)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := pagination.DecodeCursor("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// valid base64 but not valid JSON
	_, err := pagination.DecodeCursor("bm90LWpzb24=")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	_, err := pagination.DecodeCursor("")
	if err == nil {
		t.Fatal("expected error for empty cursor")
	}
	if !errors.Is(err, domain.ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestParseListOptions_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Cursor != "" {
		t.Fatalf("expected empty cursor, got %q", opts.Cursor)
	}
	if opts.Limit != 25 {
		t.Fatalf("expected default limit 25, got %d", opts.Limit)
	}
}

func TestParseListOptions_CustomValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?cursor=abc123&limit=50", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Cursor != "abc123" {
		t.Fatalf("expected cursor 'abc123', got %q", opts.Cursor)
	}
	if opts.Limit != 50 {
		t.Fatalf("expected limit 50, got %d", opts.Limit)
	}
}

func TestParseListOptions_LimitCappedAt100(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=999", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Limit != 100 {
		t.Fatalf("expected capped limit 100, got %d", opts.Limit)
	}
}

func TestParseListOptions_InvalidLimitUsesDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=notanumber", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Limit != 25 {
		t.Fatalf("expected default limit 25, got %d", opts.Limit)
	}
}

func TestParseListOptions_ZeroLimitUsesDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=0", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Limit != 25 {
		t.Fatalf("expected default limit 25, got %d", opts.Limit)
	}
}

func TestParseListOptions_NegativeLimitUsesDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=-5", nil)
	c, _ := echotest.ContextConfig{
		Request: req,
	}.ToContextRecorder(t)

	opts := pagination.ParseListOptions(c)
	if opts.Limit != 25 {
		t.Fatalf("expected default limit 25, got %d", opts.Limit)
	}
}
