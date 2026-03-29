package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/port"
)

func TestEncodeDecode_RoundTrip(t *testing.T) {
	id := uuid.New()
	cursor := EncodeCursor(id)
	got, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor() error: %v", err)
	}
	if got != id {
		t.Fatalf("round-trip mismatch: want %s, got %s", id, got)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeCursor_WrongLength(t *testing.T) {
	_, err := DecodeCursor("AQID") // 3 bytes
	if err == nil {
		t.Fatal("expected error for wrong-length cursor")
	}
}

func TestApplyPagination_Defaults(t *testing.T) {
	limit, afterID, err := ApplyPagination(port.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != defaultLimit {
		t.Errorf("want default limit %d, got %d", defaultLimit, limit)
	}
	if afterID != nil {
		t.Error("want nil afterID for empty cursor")
	}
}

func TestApplyPagination_MaxLimit(t *testing.T) {
	limit, _, err := ApplyPagination(port.ListOptions{Limit: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != maxLimit {
		t.Errorf("want max limit %d, got %d", maxLimit, limit)
	}
}

func TestApplyPagination_CustomLimit(t *testing.T) {
	limit, _, err := ApplyPagination(port.ListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 50 {
		t.Errorf("want limit 50, got %d", limit)
	}
}

func TestApplyPagination_ValidCursor(t *testing.T) {
	id := uuid.New()
	cursor := EncodeCursor(id)
	limit, afterID, err := ApplyPagination(port.ListOptions{Cursor: cursor, Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 10 {
		t.Errorf("want limit 10, got %d", limit)
	}
	if afterID == nil || *afterID != id {
		t.Errorf("want afterID %s, got %v", id, afterID)
	}
}

func TestApplyPagination_InvalidCursor(t *testing.T) {
	_, _, err := ApplyPagination(port.ListOptions{Cursor: "bad-cursor"})
	if err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}

func TestApplyPagination_ZeroLimit(t *testing.T) {
	limit, _, err := ApplyPagination(port.ListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != defaultLimit {
		t.Errorf("want default limit %d for zero, got %d", defaultLimit, limit)
	}
}

func TestApplyPagination_NegativeLimit(t *testing.T) {
	limit, _, err := ApplyPagination(port.ListOptions{Limit: -5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != defaultLimit {
		t.Errorf("want default limit %d for negative, got %d", defaultLimit, limit)
	}
}
