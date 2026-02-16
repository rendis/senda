package tracking

import (
	"strings"
	"testing"
)

func TestNewTrackingID_HasPrefix(t *testing.T) {
	id := NewTrackingID()
	if !strings.HasPrefix(id, "snd_") {
		t.Errorf("NewTrackingID() = %q, want prefix \"snd_\"", id)
	}
}

func TestNewTrackingID_CorrectLength(t *testing.T) {
	id := NewTrackingID()
	// "snd_" (4) + 28 hex chars = 32 total (fits VARCHAR(32))
	want := 32
	if len(id) != want {
		t.Errorf("len(NewTrackingID()) = %d, want %d (got %q)", len(id), want, id)
	}
}

func TestNewTrackingID_Unique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for range 100 {
		id := NewTrackingID()
		if _, ok := seen[id]; ok {
			t.Fatalf("NewTrackingID() produced duplicate: %q", id)
		}
		seen[id] = struct{}{}
	}
}
