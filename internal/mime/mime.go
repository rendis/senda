// Package sendamime provides shared MIME message construction utilities
// used by all email adapter implementations (SES, Gmail, SMTP).
// Named sendamime to avoid conflict with the standard library "mime" package.
package sendamime

import (
	"bytes"
	"fmt"
	"mime"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"

	"github.com/rendis/senda/internal/port"
)

// SafeHeaderKeyRe allows only alphanumeric characters and hyphens in custom header keys.
var SafeHeaderKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// BuildRawMessage constructs a RFC 5322 MIME message from an OutgoingEmail.
func BuildRawMessage(msg *port.OutgoingEmail) ([]byte, error) {
	var buf bytes.Buffer

	// Headers.
	headers := textproto.MIMEHeader{}
	headers.Set("From", FormatAddress(msg.From))
	headers.Set("To", FormatAddress(msg.To))
	headers.Set("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("MIME-Version", "1.0")

	if len(msg.CC) > 0 {
		addrs := make([]string, len(msg.CC))
		for i, cc := range msg.CC {
			addrs[i] = FormatAddress(cc)
		}
		headers.Set("Cc", strings.Join(addrs, ", "))
	}
	if msg.ReplyTo != nil {
		headers.Set("Reply-To", FormatAddress(*msg.ReplyTo))
	}

	// Custom headers — sanitize to prevent header injection.
	for k, v := range msg.Headers {
		if !SafeHeaderKeyRe.MatchString(k) {
			continue // skip keys with unsafe characters
		}
		headers.Set(k, SanitizeHeaderValue(v))
	}

	// Determine content type.
	hasHTML := msg.BodyHTML != ""
	hasText := msg.BodyText != ""

	if hasHTML && hasText {
		// multipart/alternative
		boundary := "senda-boundary-" + msg.TrackingID
		headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
		WriteHeaders(&buf, headers)

		fmt.Fprintf(&buf, "\r\n--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.BodyText)

		fmt.Fprintf(&buf, "\r\n--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		buf.WriteString(msg.BodyHTML)

		fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)
	} else if hasHTML {
		headers.Set("Content-Type", "text/html; charset=UTF-8")
		WriteHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyHTML)
	} else {
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		WriteHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyText)
	}

	return buf.Bytes(), nil
}

// FormatAddress formats an EmailAddress for SMTP headers.
func FormatAddress(addr port.EmailAddress) string {
	if addr.Name == "" {
		return addr.Address
	}
	return (&mail.Address{Name: addr.Name, Address: addr.Address}).String()
}

// WriteHeaders writes MIME headers to a buffer in a deterministic order for common fields.
func WriteHeaders(buf *bytes.Buffer, headers textproto.MIMEHeader) {
	order := []string{"From", "To", "Cc", "Reply-To", "Subject", "Date", "Mime-Version", "Content-Type"}
	written := make(map[string]bool)
	for _, key := range order {
		if vals, ok := headers[key]; ok {
			for _, v := range vals {
				fmt.Fprintf(buf, "%s: %s\r\n", key, v)
			}
			written[key] = true
		}
	}
	// Write remaining headers.
	for key, vals := range headers {
		if written[key] {
			continue
		}
		for _, v := range vals {
			fmt.Fprintf(buf, "%s: %s\r\n", key, v)
		}
	}
}

// SanitizeHeaderValue strips CR and LF characters to prevent header injection attacks.
func SanitizeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}
