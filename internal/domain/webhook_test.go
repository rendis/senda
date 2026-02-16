package domain

import "testing"

func TestWebhook_SubscribesTo(t *testing.T) {
	w := &Webhook{
		Events: []string{"email.sent", "email.delivered"},
	}

	if !w.SubscribesTo("email.sent") {
		t.Error("SubscribesTo(\"email.sent\") = false, want true")
	}
	if !w.SubscribesTo("email.delivered") {
		t.Error("SubscribesTo(\"email.delivered\") = false, want true")
	}
	if w.SubscribesTo("email.bounced") {
		t.Error("SubscribesTo(\"email.bounced\") = true, want false")
	}
}

func TestWebhook_SubscribesTo_Wildcard(t *testing.T) {
	w := &Webhook{
		Events: []string{"*"},
	}

	if !w.SubscribesTo("email.sent") {
		t.Error("SubscribesTo(\"email.sent\") with wildcard = false, want true")
	}
	if !w.SubscribesTo("anything") {
		t.Error("SubscribesTo(\"anything\") with wildcard = false, want true")
	}
}

func TestWebhook_SubscribesTo_Empty(t *testing.T) {
	w := &Webhook{
		Events: nil,
	}

	if w.SubscribesTo("email.sent") {
		t.Error("SubscribesTo with nil events = true, want false")
	}
}
