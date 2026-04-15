package webhook

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rendis/senda/internal/domain"
)

func TestTranslator_TranslateNotificationEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           []byte
		wantType       domain.ProviderEventType
		wantBounceType string
		wantComplaint  string
	}{
		{
			name:     "delivery",
			body:     snsEnvelope(t, "Notification", sesNotificationPayload(t, map[string]any{"notificationType": "Delivery", "mail": map[string]any{"messageId": "ses-msg-1"}, "delivery": map[string]any{"timestamp": "2026-02-17T10:00:00.000Z"}})),
			wantType: domain.EventDelivered,
		},
		{
			name:           "permanent bounce",
			body:           snsEnvelope(t, "Notification", sesNotificationPayload(t, map[string]any{"notificationType": "Bounce", "mail": map[string]any{"messageId": "ses-msg-2"}, "bounce": map[string]any{"bounceType": "Permanent", "timestamp": "2026-02-17T10:00:00.000Z", "bouncedRecipients": []map[string]any{{"emailAddress": "alice@example.com"}}}})),
			wantType:       domain.EventBounced,
			wantBounceType: "hard",
		},
		{
			name:          "complaint",
			body:          snsEnvelope(t, "Notification", sesNotificationPayload(t, map[string]any{"notificationType": "Complaint", "mail": map[string]any{"messageId": "ses-msg-3"}, "complaint": map[string]any{"complaintFeedbackType": "abuse", "feedbackId": "fb-123", "timestamp": "2026-02-17T10:00:00.000Z", "complainedRecipients": []map[string]any{{"emailAddress": "alice@example.com"}}}})),
			wantType:      domain.EventComplained,
			wantComplaint: "abuse",
		},
	}

	translator := NewTranslator()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := translator.Translate(tt.body)
			if err != nil {
				t.Fatalf("Translate() error = %v", err)
			}
			assertTranslatedNotification(t, got, tt.body, tt.wantType, tt.wantBounceType, tt.wantComplaint)
		})
	}
}

func assertTranslatedNotification(t *testing.T, got *ParsedMessage, rawBody []byte, wantType domain.ProviderEventType, wantBounceType, wantComplaint string) {
	t.Helper()

	if got.Kind != KindNotification {
		t.Fatalf("expected notification kind, got %q", got.Kind)
	}
	if got.Event == nil {
		t.Fatal("expected normalized provider event")
	}
	if got.Event.Type != wantType {
		t.Fatalf("expected provider event type %q, got %q", wantType, got.Event.Type)
	}
	if !got.Event.Timestamp.Equal(time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected parsed SES timestamp, got %s", got.Event.Timestamp)
	}
	assertBounceDetail(t, got.Event, wantBounceType)
	assertComplaintDetail(t, got.Event, wantComplaint)
	if string(got.Event.RawPayload) != string(rawBody) {
		t.Fatal("expected raw payload to preserve original SNS body")
	}
}

func assertBounceDetail(t *testing.T, event *domain.ProviderEvent, want string) {
	t.Helper()

	if want == "" {
		return
	}
	if event.BounceDetail == nil || event.BounceDetail.BounceType != want {
		t.Fatalf("expected bounce detail %q, got %#v", want, event.BounceDetail)
	}
}

func assertComplaintDetail(t *testing.T, event *domain.ProviderEvent, want string) {
	t.Helper()

	if want == "" {
		return
	}
	if event.ComplaintDetail == nil || event.ComplaintDetail.ComplaintType != want {
		t.Fatalf("expected complaint detail %q, got %#v", want, event.ComplaintDetail)
	}
}

func TestTranslator_TranslateSubscriptionConfirmation(t *testing.T) {
	t.Parallel()

	body := snsEnvelope(t, "SubscriptionConfirmation", "ignored")
	translator := NewTranslator()

	got, err := translator.Translate(body)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if got.Kind != KindSubscriptionConfirmation {
		t.Fatalf("expected subscription confirmation kind, got %q", got.Kind)
	}
	if got.SubscribeURL == "" {
		t.Fatal("expected subscribe URL to be preserved")
	}
	if got.Event != nil {
		t.Fatal("expected subscription confirmation to avoid building a provider event")
	}
}

func snsEnvelope(t *testing.T, messageType, message string) []byte {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"Type":             messageType,
		"MessageId":        "sns-msg-1",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          message,
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
		"SubscribeURL":     "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&TopicArn=arn:aws:sns:us-east-1:123456789012:SES-Events&Token=abc123token",
	})
	if err != nil {
		t.Fatalf("marshal SNS envelope: %v", err)
	}
	return payload
}

func sesNotificationPayload(t *testing.T, payload map[string]any) string {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal SES payload: %v", err)
	}
	return string(body)
}
