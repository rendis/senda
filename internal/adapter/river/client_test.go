package river

import (
	"testing"

	"github.com/google/uuid"
	goriver "github.com/riverqueue/river"
	"github.com/rendis/senda/internal/port"
)

// --- Job Args tests ---

func TestSendJobArgs_Kind(t *testing.T) {
	args := SendJobArgs{}
	if got := args.Kind(); got != "send_email" {
		t.Errorf("Kind() = %q, want %q", got, "send_email")
	}
}

func TestSendJobArgs_InsertOpts(t *testing.T) {
	args := SendJobArgs{Priority: 1}
	opts := args.InsertOpts()
	if opts.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", opts.MaxAttempts)
	}
	if opts.Priority != 1 {
		t.Errorf("Priority = %d, want 1", opts.Priority)
	}
	if opts.Queue != "send" {
		t.Errorf("Queue = %q, want %q", opts.Queue, "send")
	}
}

func TestWebhookJobArgs_Kind(t *testing.T) {
	args := WebhookJobArgs{}
	if got := args.Kind(); got != "deliver_webhook" {
		t.Errorf("Kind() = %q, want %q", got, "deliver_webhook")
	}
}

func TestWebhookJobArgs_InsertOpts(t *testing.T) {
	args := WebhookJobArgs{}
	opts := args.InsertOpts()
	if opts.MaxAttempts != 6 {
		t.Errorf("MaxAttempts = %d, want 6", opts.MaxAttempts)
	}
	if opts.Queue != "webhook" {
		t.Errorf("Queue = %q, want %q", opts.Queue, "webhook")
	}
}

// Verify compile-time interface conformance.
func TestJobArgsInterfaces(t *testing.T) {
	// All job args must implement river.JobArgs.
	var _ goriver.JobArgs = SendJobArgs{}
	var _ goriver.JobArgs = WebhookJobArgs{}
}

func TestClientImplementsJobQueue(t *testing.T) {
	// Compile-time check that Client satisfies port.JobQueue.
	var _ port.JobQueue = (*Client)(nil)
}

func TestSendJobArgs_FieldMapping(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	args := SendJobArgs{
		EmailID:    id,
		TrackingID: "trk_abc123",
		AdapterID:  adapterID,
		Priority:   1,
	}
	if args.EmailID != id {
		t.Error("EmailID mismatch")
	}
	if args.TrackingID != "trk_abc123" {
		t.Error("TrackingID mismatch")
	}
	if args.AdapterID != adapterID {
		t.Error("AdapterID mismatch")
	}
	if args.Priority != 1 {
		t.Error("Priority mismatch")
	}
}

func TestWebhookJobArgs_FieldMapping(t *testing.T) {
	whID := uuid.Must(uuid.NewV7())
	payload := []byte(`{"event":"delivered"}`)
	args := WebhookJobArgs{
		WebhookID:  whID,
		EventType:  "email.delivered",
		Payload:    payload,
		RetryCount: 2,
	}
	if args.WebhookID != whID {
		t.Error("WebhookID mismatch")
	}
	if args.EventType != "email.delivered" {
		t.Error("EventType mismatch")
	}
	if string(args.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if args.RetryCount != 2 {
		t.Error("RetryCount mismatch")
	}
}
