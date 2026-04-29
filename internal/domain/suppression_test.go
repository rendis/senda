package domain

import "testing"

func TestSuppressionReason_Unsubscribe(t *testing.T) {
	if string(SuppressionUnsubscribe) != "unsubscribe" {
		t.Fatalf("SuppressionUnsubscribe constant must serialize as 'unsubscribe', got %q", SuppressionUnsubscribe)
	}
}
