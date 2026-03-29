package smtp

import (
	"strings"
	"testing"

	"github.com/rendis/senda/internal/port"
	"github.com/stretchr/testify/require"
)

func TestBuildRawMessage_PlainText(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Name: "Senda", Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Hello",
		BodyText:   "Hello World",
		TrackingID: "trk-001",
	}

	raw, err := buildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "From: \"Senda\" <noreply@test.com>")
	require.Contains(t, body, "To: user@example.com")
	require.Contains(t, body, "Content-Type: text/plain; charset=UTF-8")
	require.Contains(t, body, "Hello World")
}

func TestBuildRawMessage_HTMLOnly(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Test",
		BodyHTML:   "<h1>Hi</h1>",
		TrackingID: "trk-002",
	}

	raw, err := buildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "Content-Type: text/html; charset=UTF-8")
	require.Contains(t, body, "<h1>Hi</h1>")
}

func TestBuildRawMessage_Multipart(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Test",
		BodyHTML:   "<h1>Hi</h1>",
		BodyText:   "Hi",
		TrackingID: "trk-003",
	}

	raw, err := buildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "multipart/alternative")
	require.Contains(t, body, "text/plain")
	require.Contains(t, body, "text/html")
	require.Contains(t, body, "Hi")
	require.Contains(t, body, "<h1>Hi</h1>")
}

func TestBuildRawMessage_WithCCAndReplyTo(t *testing.T) {
	replyTo := port.EmailAddress{Address: "reply@test.com"}
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		CC:         []port.EmailAddress{{Address: "cc@example.com"}},
		ReplyTo:    &replyTo,
		Subject:    "Test",
		BodyText:   "text",
		TrackingID: "trk-004",
	}

	raw, err := buildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "Cc: cc@example.com")
	require.Contains(t, body, "Reply-To: reply@test.com")
}

func TestSanitizeHeaderValue_StripsNewlines(t *testing.T) {
	result := sanitizeHeaderValue("value\r\ninjected: bad")
	require.Equal(t, "valueinjected: bad", result)
}

func TestAllRecipients(t *testing.T) {
	msg := &port.OutgoingEmail{
		To:  port.EmailAddress{Address: "to@test.com"},
		CC:  []port.EmailAddress{{Address: "cc@test.com"}},
		BCC: []port.EmailAddress{{Address: "bcc@test.com"}},
	}

	addrs := allRecipients(msg)
	require.Equal(t, []string{"to@test.com", "cc@test.com", "bcc@test.com"}, addrs)
}

func TestBuildRawMessage_HeaderInjectionPrevented(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Test",
		BodyText:   "text",
		TrackingID: "trk-005",
		Headers: map[string]string{
			"X-Custom":          "safe",
			"X-Bad\r\nBcc":     "injected",
			"X-Valid-Header":    "ok\r\nInjected: bad",
		},
	}

	raw, err := buildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "X-Custom: safe")
	require.Contains(t, body, "X-Valid-Header: okInjected: bad")
	// Key with CRLF should be rejected by regex
	require.False(t, strings.Contains(body, "X-Bad"))
}

func TestAdapter_Name(t *testing.T) {
	a := NewAdapter("localhost", 1025)
	require.Equal(t, "smtp", a.Name())
}
