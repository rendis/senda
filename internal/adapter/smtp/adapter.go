package smtp

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/senda-app/senda/internal/port"
)

// safeHeaderKeyRe allows only alphanumeric characters and hyphens in custom header keys.
var safeHeaderKeyRe = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// Adapter implements port.EmailSender using plain SMTP.
// Designed for local development with Mailpit or similar SMTP test servers.
type Adapter struct {
	host string
	port int
}

// NewAdapter creates a new SMTP adapter.
func NewAdapter(host string, port int) *Adapter {
	return &Adapter{host: host, port: port}
}

// Send delivers an email via SMTP.
func (a *Adapter) Send(_ context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := buildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("smtp: build message: %w", err)
	}

	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	recipients := allRecipients(msg)

	if err := smtp.SendMail(addr, nil, msg.From.Address, recipients, rawMsg); err != nil {
		return "", fmt.Errorf("smtp: send: %w", err)
	}

	providerID := fmt.Sprintf("<trk-%s@senda>", msg.TrackingID)
	return providerID, nil
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "smtp" }

// HealthCheck verifies the SMTP server is reachable.
func (a *Adapter) HealthCheck(_ context.Context) error {
	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("smtp: health check: %w", err)
	}
	_ = conn.Close()
	return nil
}

// Compile-time interface check.
var _ port.EmailSender = (*Adapter)(nil)

// buildRawMessage constructs a RFC 5322 MIME message from an OutgoingEmail.
func buildRawMessage(msg *port.OutgoingEmail) ([]byte, error) {
	var buf bytes.Buffer

	headers := textproto.MIMEHeader{}
	headers.Set("From", formatAddress(msg.From))
	headers.Set("To", formatAddress(msg.To))
	headers.Set("Subject", mime.QEncoding.Encode("UTF-8", msg.Subject))
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("MIME-Version", "1.0")

	if len(msg.CC) > 0 {
		addrs := make([]string, len(msg.CC))
		for i, cc := range msg.CC {
			addrs[i] = formatAddress(cc)
		}
		headers.Set("Cc", strings.Join(addrs, ", "))
	}
	if msg.ReplyTo != nil {
		headers.Set("Reply-To", formatAddress(*msg.ReplyTo))
	}

	for k, v := range msg.Headers {
		if !safeHeaderKeyRe.MatchString(k) {
			continue
		}
		headers.Set(k, sanitizeHeaderValue(v))
	}

	hasHTML := msg.BodyHTML != ""
	hasText := msg.BodyText != ""

	if hasHTML && hasText {
		boundary := "senda-boundary-" + msg.TrackingID
		headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", boundary))
		writeHeaders(&buf, headers)

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
		writeHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyHTML)
	} else {
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		writeHeaders(&buf, headers)
		buf.WriteString("\r\n")
		buf.WriteString(msg.BodyText)
	}

	return buf.Bytes(), nil
}

func formatAddress(addr port.EmailAddress) string {
	if addr.Name == "" {
		return addr.Address
	}
	return (&mail.Address{Name: addr.Name, Address: addr.Address}).String()
}

func writeHeaders(buf *bytes.Buffer, headers textproto.MIMEHeader) {
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
	for key, vals := range headers {
		if written[key] {
			continue
		}
		for _, v := range vals {
			fmt.Fprintf(buf, "%s: %s\r\n", key, v)
		}
	}
}

func sanitizeHeaderValue(v string) string {
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

func allRecipients(msg *port.OutgoingEmail) []string {
	var addrs []string
	addrs = append(addrs, msg.To.Address)
	for _, cc := range msg.CC {
		addrs = append(addrs, cc.Address)
	}
	for _, bcc := range msg.BCC {
		addrs = append(addrs, bcc.Address)
	}
	return addrs
}
