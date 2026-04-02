package awssim

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type publishCall struct {
	topicARN string
	message  string
}

type stubPublisher struct {
	calls []publishCall
}

func (s *stubPublisher) Publish(_ context.Context, topicARN, message string) error {
	s.calls = append(s.calls, publishCall{topicARN: topicARN, message: message})
	return nil
}

type stubSubscriptions struct {
	endpoints []string
}

func (s *stubSubscriptions) HTTPSubscriptions(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), s.endpoints...), nil
}

type deliverCall struct {
	endpoint string
	envelope []byte
}

type stubDeliverer struct {
	calls []deliverCall
}

func (s *stubDeliverer) Deliver(_ context.Context, endpoint string, envelope []byte) error {
	s.calls = append(s.calls, deliverCall{endpoint: endpoint, envelope: append([]byte(nil), envelope...)})
	return nil
}

func TestBridge_CreateDeleteEventDestination(t *testing.T) {
	bridge, err := NewBridge(Config{BackendBaseURL: "http://example.com"})
	if err != nil {
		t.Fatalf("NewBridge() error = %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v2/email/configuration-sets/senda-1234/event-destinations", bytes.NewBufferString(`{
		"EventDestinationName":"senda-events",
		"EventDestination":{"Enabled":true,"MatchingEventTypes":["SEND","DELIVERY"],"SnsDestination":{"TopicArn":"arn:aws:sns:us-east-1:000000000000:test"}}}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create event destination status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	stateReq := httptest.NewRequest(http.MethodGet, "/_aws-sim/state", nil)
	stateRec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(stateRec, stateReq)
	if stateRec.Code != http.StatusOK {
		t.Fatalf("state status = %d, body = %s", stateRec.Code, stateRec.Body.String())
	}

	var state State
	if err := json.Unmarshal(stateRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if len(state.EventDestinations) != 1 {
		t.Fatalf("event destinations len = %d, want 1", len(state.EventDestinations))
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/v2/email/configuration-sets/senda-1234/event-destinations/senda-events", nil)
	deleteRec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete event destination status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	stateRec = httptest.NewRecorder()
	bridge.Handler().ServeHTTP(stateRec, stateReq)
	if err := json.Unmarshal(stateRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("unmarshal state after delete: %v", err)
	}
	if len(state.EventDestinations) != 0 {
		t.Fatalf("event destinations len = %d, want 0", len(state.EventDestinations))
	}
}

func TestBridge_SendEmailTracksProviderMessageID(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/email/outbound-emails" {
			t.Fatalf("backend path = %s, want /v2/email/outbound-emails", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"MessageId":"ministack-msg-123"}`)
	}))
	defer backend.Close()

	bridge, err := NewBridge(Config{BackendBaseURL: backend.URL})
	if err != nil {
		t.Fatalf("NewBridge() error = %v", err)
	}

	sendReq := httptest.NewRequest(http.MethodPost, "/v2/email/outbound-emails", bytes.NewBufferString(`{
		"ConfigurationSetName":"senda-1234",
		"Destination":{"ToAddresses":["recipient@test.example.com"]},
		"Content":{"Raw":{"Data":"SGVsbG8="}}
	}`))
	sendReq.Header.Set("Content-Type", "application/json")
	sendRec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("send status = %d, body = %s", sendRec.Code, sendRec.Body.String())
	}

	stateReq := httptest.NewRequest(http.MethodGet, "/_aws-sim/state", nil)
	stateRec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(stateRec, stateReq)

	var state State
	if err := json.Unmarshal(stateRec.Body.Bytes(), &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}

	msg, ok := state.Messages["ministack-msg-123"]
	if !ok {
		t.Fatalf("tracked message not found in state")
	}
	if msg.ConfigurationSetName != "senda-1234" {
		t.Fatalf("configuration set = %q, want senda-1234", msg.ConfigurationSetName)
	}
	if len(msg.Recipients) != 1 || msg.Recipients[0] != "recipient@test.example.com" {
		t.Fatalf("recipients = %#v", msg.Recipients)
	}
}

func TestBridge_ControlEmitPublishesNotification(t *testing.T) {
	publisher := &stubPublisher{}
	subscriptions := &stubSubscriptions{endpoints: []string{"http://senda:8080/api/v1/webhooks/ses/inbound"}}
	deliverer := &stubDeliverer{}
	bridge, err := NewBridge(Config{
		BackendBaseURL: "http://example.com",
		Publisher:      publisher,
		Subscriptions:  subscriptions,
		Deliverer:      deliverer,
	})
	if err != nil {
		t.Fatalf("NewBridge() error = %v", err)
	}

	bridge.eventDestinations[eventDestinationKey("senda-1234", "senda-events")] = EventDestinationRecord{
		ConfigurationSetName: "senda-1234",
		EventDestinationName: "senda-events",
		TopicARN:             "arn:aws:sns:us-east-1:000000000000:senda-events",
		MatchingEventTypes:   []string{"SEND", "DELIVERY", "BOUNCE", "COMPLAINT"},
		Enabled:              true,
	}
	bridge.messages["provider-123"] = MessageRecord{
		ProviderMessageID:    "provider-123",
		ConfigurationSetName: "senda-1234",
		Recipients:           []string{"recipient@test.example.com"},
	}

	req := httptest.NewRequest(http.MethodPost, "/_aws-sim/control/ses-events", bytes.NewBufferString(`{
		"notification_type":"Delivery",
		"provider_message_id":"provider-123"
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("emit status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(publisher.calls) != 1 {
		t.Fatalf("publish calls = %d, want 1", len(publisher.calls))
	}
	if len(deliverer.calls) != 1 {
		t.Fatalf("deliver calls = %d, want 1", len(deliverer.calls))
	}
	if publisher.calls[0].topicARN != "arn:aws:sns:us-east-1:000000000000:senda-events" {
		t.Fatalf("publish topic = %q", publisher.calls[0].topicARN)
	}

	var notification map[string]any
	if err := json.Unmarshal([]byte(publisher.calls[0].message), &notification); err != nil {
		t.Fatalf("unmarshal published notification: %v", err)
	}
	if notification["notificationType"] != "Delivery" {
		t.Fatalf("notificationType = %v", notification["notificationType"])
	}

	var envelope map[string]any
	if err := json.Unmarshal(deliverer.calls[0].envelope, &envelope); err != nil {
		t.Fatalf("unmarshal delivered envelope: %v", err)
	}
	if envelope["Type"] != "Notification" {
		t.Fatalf("envelope type = %v, want Notification", envelope["Type"])
	}
}
