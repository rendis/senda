package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	sendamime "github.com/rendis/senda/internal/mime"
	"github.com/rendis/senda/internal/port"
)

// TLSMode controls the SMTP transport security mode.
type TLSMode string

const (
	TLSModeNone        TLSMode = "none"
	TLSModeStartTLS    TLSMode = "starttls"
	TLSModeImplicitTLS TLSMode = "implicit_tls"
)

const authModeLogin = "login"

// Config defines relay-only SMTP adapter configuration.
type Config struct {
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	TLSMode  TLSMode `json:"tls_mode"`
	AuthMode string  `json:"auth_mode,omitempty"`
	Username string  `json:"username,omitempty"`
	Password string  `json:"password,omitempty"`
}

// Validate checks the SMTP relay configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("missing SMTP host")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid SMTP port")
	}
	switch c.TLSMode {
	case TLSModeNone, TLSModeStartTLS, TLSModeImplicitTLS:
	case "":
		return fmt.Errorf("missing SMTP tls_mode")
	default:
		return fmt.Errorf("invalid SMTP tls_mode %q", c.TLSMode)
	}
	if c.AuthMode != "" && c.AuthMode != "plain" && c.AuthMode != authModeLogin {
		return fmt.Errorf("invalid SMTP auth_mode %q", c.AuthMode)
	}
	if (c.Username == "") != (c.Password == "") {
		return fmt.Errorf("smtp username and password must be provided together")
	}
	return nil
}

// NewAdapterFromConfig creates a configured SMTP adapter.
func NewAdapterFromConfig(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Adapter{cfg: cfg}, nil
}

// Adapter implements port.EmailSender using SMTP relays.
type Adapter struct {
	cfg Config
}

// NewAdapter creates a new SMTP adapter.
func NewAdapter(host string, port int) *Adapter {
	return &Adapter{cfg: Config{Host: host, Port: port, TLSMode: TLSModeNone}}
}

// Send delivers an email via SMTP.
func (a *Adapter) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	rawMsg, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		return "", fmt.Errorf("smtp: build message: %w", err)
	}

	addr := net.JoinHostPort(a.cfg.Host, strconv.Itoa(a.cfg.Port))
	recipients := allRecipients(msg)
	auth := a.auth()

	switch a.cfg.TLSMode {
	case TLSModeImplicitTLS:
		err = a.sendImplicitTLS(ctx, addr, auth, msg.From.Address, recipients, rawMsg)
	default:
		err = smtp.SendMail(addr, auth, msg.From.Address, recipients, rawMsg)
	}
	if err != nil {
		return "", fmt.Errorf("smtp: send: %w", err)
	}

	return fmt.Sprintf("<trk-%s@senda>", msg.TrackingID), nil
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "smtp" }

// HealthCheck verifies the SMTP server is reachable.
func (a *Adapter) HealthCheck(_ context.Context) error {
	addr := net.JoinHostPort(a.cfg.Host, strconv.Itoa(a.cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("smtp: health check: %w", err)
	}
	_ = conn.Close()
	return nil
}

// Compile-time interface check.
var _ port.EmailSender = (*Adapter)(nil)

func (a *Adapter) auth() smtp.Auth {
	if a.cfg.Username == "" && a.cfg.Password == "" {
		return nil
	}
	if a.cfg.AuthMode == authModeLogin {
		return loginAuth{username: a.cfg.Username, password: a.cfg.Password}
	}
	return smtp.PlainAuth("", a.cfg.Username, a.cfg.Password, a.cfg.Host)
}

type loginAuth struct {
	username string
	password string
}

func (a loginAuth) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(string(fromServer))
	if strings.Contains(prompt, "password") {
		return []byte(a.password), nil
	}
	return []byte(a.username), nil
}

func (a *Adapter) sendImplicitTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, rawMsg []byte) error {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: a.cfg.Host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, a.cfg.Host)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	return sendWithClient(client, auth, from, to, rawMsg)
}

func sendWithClient(client *smtp.Client, auth smtp.Auth, from string, to []string, rawMsg []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(rawMsg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
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
