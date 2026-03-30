package smtp

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	sendamime "github.com/rendis/senda/internal/mime"
	"github.com/rendis/senda/internal/port"
)

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
	rawMsg, err := sendamime.BuildRawMessage(msg)
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
