package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/labstack/echo/v5/echotest"
	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

func testAppLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewSNSHTTPClient_UsesExplicitTimeout(t *testing.T) {
	client := newSNSHTTPClient()
	if client == nil {
		t.Fatal("expected client")
	}
	if client.Timeout <= 0 {
		t.Fatalf("expected positive timeout, got %v", client.Timeout)
	}
}

func TestBuildSESWebhookHandler_UsesConfiguredSNSBinding(t *testing.T) {
	cfg := &config.Config{
		SNS: config.SNSConfig{
			SkipSignatureVerification: true,
			ExpectedTopicArn:          "arn:aws:sns:us-east-1:123456789012:SES-Events",
			ExpectedAccountID:         "123456789012",
		},
	}

	h, err := buildSESWebhookHandler(cfg, nil, testAppLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := map[string]any{
		"Type":             "Notification",
		"MessageId":        "sns-msg-bad-binding",
		"TopicArn":         "arn:aws:sns:us-east-1:999999999999:SES-Events",
		"Message":          `{"notificationType":"Delivery","mail":{"messageId":"ses-msg-id-001"},"delivery":{"timestamp":"2026-02-17T10:00:00.000Z"}}`,
		"Timestamp":        "2026-02-17T10:00:01.000Z",
		"SignatureVersion": "1",
		"Signature":        "sig==",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	body, _ := jsonMarshal(t, msg)

	c, rec := echotest.ContextConfig{
		Request: httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/ses/inbound", bytes.NewReader(body)),
	}.ToContextRecorder(t)

	if err := h.HandleInbound(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unexpected SNS destination, got %d", rec.Code)
	}
}

func TestBuildMediaHandler_UsesConfiguredAllowlist(t *testing.T) {
	cfg := &config.Config{
		Media: config.MediaConfig{
			ThumbnailAllowedHosts:    []string{"allowed.example"},
			ThumbnailCacheTTL:        time.Minute,
			ThumbnailCacheMaxEntries: 1,
			ThumbnailFetchTimeout:    2 * time.Second,
		},
	}

	h := buildMediaHandler(cfg, testAppLogger())
	req := httptest.NewRequest(http.MethodGet, "/public/video-thumbnail?url="+url.QueryEscape("https://blocked.example/thumb.jpg"), nil)
	c, rec := echotest.ContextConfig{Request: req}.ToContextRecorder(t)

	if err := h.HandleVideoThumbnail(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-allowlisted media host, got %d", rec.Code)
	}
}

func TestNewTestSendSenderFactory_UsesAssignedAdapterConfig(t *testing.T) {
	globalSender := namedEmailSender{name: "global-smtp"}
	factory := newTestSendSenderFactory(globalSender)

	sender, err := factory(
		context.Background(),
		&domain.Adapter{AdapterType: domain.AdapterTypeSMTP},
		[]byte(`{"host":"127.0.0.1","port":2525,"tls_mode":"none","auth_mode":"plain","username":"user","password":"pass"}`),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.Name() == globalSender.Name() {
		t.Fatalf("expected template test-send to use the assigned adapter config, got global sender %q", sender.Name())
	}
}

type namedEmailSender struct {
	name string
}

func (s namedEmailSender) Send(context.Context, *port.OutgoingEmail) (string, error) {
	return "", nil
}

func (s namedEmailSender) Name() string {
	return s.name
}

func (s namedEmailSender) HealthCheck(context.Context) error {
	return nil
}

func jsonMarshal(t *testing.T, v any) ([]byte, error) {
	t.Helper()
	return json.Marshal(v)
}
