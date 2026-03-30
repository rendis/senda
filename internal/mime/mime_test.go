package sendamime_test

import (
	"strings"
	"testing"

	sendamime "github.com/rendis/senda/internal/mime"
	"github.com/rendis/senda/internal/port"
)

func TestBuildRawMessage_HTMLAndText(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Name: "Sender", Address: "sender@example.com"},
		To:         port.EmailAddress{Name: "Recipient", Address: "recipient@example.com"},
		Subject:    "Test Subject",
		BodyHTML:   "<h1>Hello</h1>",
		BodyText:   "Hello",
		TrackingID: "trk-001",
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage returned error: %v", err)
	}

	body := string(raw)

	if !strings.Contains(body, "multipart/alternative") {
		t.Error("expected multipart/alternative Content-Type for HTML+text message")
	}
	if !strings.Contains(body, "senda-boundary-trk-001") {
		t.Error("expected boundary to contain tracking ID")
	}
	if !strings.Contains(body, "<h1>Hello</h1>") {
		t.Error("expected HTML body in message")
	}
	if !strings.Contains(body, "Hello") {
		t.Error("expected text body in message")
	}
	if !strings.Contains(body, "From:") {
		t.Error("expected From header")
	}
	if !strings.Contains(body, "To:") {
		t.Error("expected To header")
	}
	if !strings.Contains(body, "Subject:") {
		t.Error("expected Subject header")
	}
}

func TestBuildRawMessage_HTMLOnly(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "sender@example.com"},
		To:         port.EmailAddress{Address: "recipient@example.com"},
		Subject:    "HTML Only",
		BodyHTML:   "<p>HTML only</p>",
		TrackingID: "trk-002",
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage returned error: %v", err)
	}

	body := string(raw)

	if !strings.Contains(body, "text/html; charset=UTF-8") {
		t.Error("expected text/html Content-Type for HTML-only message")
	}
	if strings.Contains(body, "multipart/alternative") {
		t.Error("did not expect multipart/alternative for HTML-only message")
	}
	if !strings.Contains(body, "<p>HTML only</p>") {
		t.Error("expected HTML body in message")
	}
}

func TestBuildRawMessage_TextOnly(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "sender@example.com"},
		To:         port.EmailAddress{Address: "recipient@example.com"},
		Subject:    "Text Only",
		BodyText:   "Plain text only",
		TrackingID: "trk-003",
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage returned error: %v", err)
	}

	body := string(raw)

	if !strings.Contains(body, "text/plain; charset=UTF-8") {
		t.Error("expected text/plain Content-Type for text-only message")
	}
	if strings.Contains(body, "multipart/alternative") {
		t.Error("did not expect multipart/alternative for text-only message")
	}
	if !strings.Contains(body, "Plain text only") {
		t.Error("expected text body in message")
	}
}

func TestFormatAddress_WithName(t *testing.T) {
	addr := port.EmailAddress{Name: "John Doe", Address: "john@example.com"}
	result := sendamime.FormatAddress(addr)
	if !strings.Contains(result, "John Doe") {
		t.Errorf("expected name in formatted address, got: %s", result)
	}
	if !strings.Contains(result, "john@example.com") {
		t.Errorf("expected email in formatted address, got: %s", result)
	}
}

func TestFormatAddress_WithoutName(t *testing.T) {
	addr := port.EmailAddress{Address: "john@example.com"}
	result := sendamime.FormatAddress(addr)
	if result != "john@example.com" {
		t.Errorf("expected bare email address, got: %s", result)
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal value", "normal value"},
		{"has\rnewline", "hasnewline"},
		{"has\nnewline", "hasnewline"},
		{"has\r\nboth", "hasboth"},
	}
	for _, tt := range tests {
		got := sendamime.SanitizeHeaderValue(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeHeaderValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafeHeaderKeyRe(t *testing.T) {
	valid := []string{"X-Custom-Header", "Content-Type", "X123"}
	for _, k := range valid {
		if !sendamime.SafeHeaderKeyRe.MatchString(k) {
			t.Errorf("SafeHeaderKeyRe should match %q", k)
		}
	}
	invalid := []string{"X Header", "X\nHeader", "X:Header", "X;Header"}
	for _, k := range invalid {
		if sendamime.SafeHeaderKeyRe.MatchString(k) {
			t.Errorf("SafeHeaderKeyRe should NOT match %q", k)
		}
	}
}
