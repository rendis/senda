package smtp

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"

	sendamime "github.com/rendis/senda/internal/mime"
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

	raw, err := sendamime.BuildRawMessage(msg)
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

	raw, err := sendamime.BuildRawMessage(msg)
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

	raw, err := sendamime.BuildRawMessage(msg)
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

	raw, err := sendamime.BuildRawMessage(msg)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "Cc: cc@example.com")
	require.Contains(t, body, "Reply-To: reply@test.com")
}

func TestSanitizeHeaderValue_StripsNewlines(t *testing.T) {
	result := sendamime.SanitizeHeaderValue("value\r\ninjected: bad")
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

func TestConfigValidate_PlainSMTP(t *testing.T) {
	cfg := Config{
		Host:    "localhost",
		Port:    1025,
		TLSMode: TLSModeNone,
	}

	require.NoError(t, cfg.Validate())
}

func TestConfigValidate_AuthenticatedSMTPRequiresUsernameAndPasswordTogether(t *testing.T) {
	cfg := Config{
		Host:     "smtp.example.com",
		Port:     587,
		TLSMode:  TLSModeStartTLS,
		Username: "apikey",
	}

	require.ErrorContains(t, cfg.Validate(), "smtp username and password must be provided together")
}

func TestConfigValidate_RejectsUnknownTLSMode(t *testing.T) {
	cfg := Config{
		Host:    "smtp.example.com",
		Port:    587,
		TLSMode: TLSMode("ssl-ish"),
	}

	require.ErrorContains(t, cfg.Validate(), "invalid SMTP tls_mode")
}

func TestAdapterSend_TLSModeNoneDoesNotStartTLS(t *testing.T) {
	server := startFakeSMTPServer(t, fakeSMTPOptions{advertiseStartTLS: true})

	adapter, err := NewAdapterFromConfig(Config{
		Host:    server.host,
		Port:    server.port,
		TLSMode: TLSModeNone,
	})
	require.NoError(t, err)

	_, err = adapter.Send(context.Background(), testOutgoingEmail())
	require.NoError(t, err)
	require.NotContains(t, server.commands(), "STARTTLS")
}

func TestAdapterSend_TLSModeStartTLSRequiresServerSupport(t *testing.T) {
	server := startFakeSMTPServer(t, fakeSMTPOptions{})

	adapter, err := NewAdapterFromConfig(Config{
		Host:    server.host,
		Port:    server.port,
		TLSMode: TLSModeStartTLS,
	})
	require.NoError(t, err)

	_, err = adapter.Send(context.Background(), testOutgoingEmail())
	require.ErrorContains(t, err, "STARTTLS")
	require.NotContains(t, server.commands(), "MAIL")
}

func TestBuildRawMessage_HeaderInjectionPrevented(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "noreply@test.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Test",
		BodyText:   "text",
		TrackingID: "trk-005",
		Headers: map[string]string{
			"X-Custom":       "safe",
			"X-Bad\r\nBcc":   "injected",
			"X-Valid-Header": "ok\r\nInjected: bad",
		},
	}

	raw, err := sendamime.BuildRawMessage(msg)
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

func testOutgoingEmail() *port.OutgoingEmail {
	return &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "sender@example.com"},
		To:         port.EmailAddress{Address: "recipient@example.com"},
		Subject:    "SMTP test",
		BodyText:   "hello",
		TrackingID: "trk-smtp-test",
	}
}

type fakeSMTPOptions struct {
	advertiseStartTLS bool
}

type fakeSMTPServer struct {
	host string
	port int

	listener net.Listener
	done     chan struct{}

	mu           sync.Mutex
	commandsSeen []string
}

func startFakeSMTPServer(t *testing.T, opts fakeSMTPOptions) *fakeSMTPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	host, portString, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portString)
	require.NoError(t, err)

	server := &fakeSMTPServer{
		host:     host,
		port:     port,
		listener: listener,
		done:     make(chan struct{}),
	}
	go server.serve(opts)

	t.Cleanup(func() {
		_ = listener.Close()
		<-server.done
	})

	return server
}

func (s *fakeSMTPServer) commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commandsSeen...)
}

func (s *fakeSMTPServer) record(command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commandsSeen = append(s.commandsSeen, command)
}

func (s *fakeSMTPServer) serve(opts fakeSMTPOptions) {
	defer close(s.done)

	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeSMTPLine(writer, "220 fake-smtp ESMTP")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)
		upper := strings.ToUpper(command)
		if idx := strings.IndexByte(upper, ' '); idx >= 0 {
			upper = upper[:idx]
		}
		s.record(upper)

		switch upper {
		case "EHLO":
			writeSMTPLine(writer, "250-fake-smtp")
			if opts.advertiseStartTLS {
				writeSMTPLine(writer, "250-STARTTLS")
			}
			writeSMTPLine(writer, "250 OK")
		case "HELO":
			writeSMTPLine(writer, "250 OK")
		case "STARTTLS":
			writeSMTPLine(writer, "454 TLS unavailable in fake server")
		case "MAIL", "RCPT":
			writeSMTPLine(writer, "250 OK")
		case "DATA":
			writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>")
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dataLine) == "." {
					break
				}
			}
			writeSMTPLine(writer, "250 queued")
		case "QUIT":
			writeSMTPLine(writer, "221 bye")
			return
		default:
			writeSMTPLine(writer, "250 OK")
		}
	}
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = writer.WriteString(line + "\r\n")
	_ = writer.Flush()
}
