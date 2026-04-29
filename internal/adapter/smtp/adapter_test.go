package smtp

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"net/textproto"
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

func TestConfigValidate_RejectsCleartextAuthForNonLoopback(t *testing.T) {
	for _, authMode := range []string{"plain", authModeLogin} {
		t.Run(authMode, func(t *testing.T) {
			cfg := Config{
				Host:     "smtp.example.com",
				Port:     2525,
				TLSMode:  TLSModeNone,
				AuthMode: authMode,
				Username: "apikey",
				Password: "secret",
			}

			require.ErrorContains(t, cfg.Validate(), "smtp cleartext auth is only allowed for loopback hosts")
		})
	}
}

func TestConfigValidate_AllowsCleartextAuthForLoopback(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "localhost"} {
		for _, authMode := range []string{"plain", authModeLogin} {
			t.Run(host+"_"+authMode, func(t *testing.T) {
				cfg := Config{
					Host:     host,
					Port:     2525,
					TLSMode:  TLSModeNone,
					AuthMode: authMode,
					Username: "apikey",
					Password: "secret",
				}

				require.NoError(t, cfg.Validate())
			})
		}
	}
}

func TestConfigValidate_RejectsUnknownTLSMode(t *testing.T) {
	cfg := Config{
		Host:    "smtp.example.com",
		Port:    587,
		TLSMode: TLSMode("ssl-ish"),
	}

	require.ErrorContains(t, cfg.Validate(), "invalid SMTP tls_mode")
}

func TestAdapter_IsPermanentSendError_ClassifiesSMTPStatus(t *testing.T) {
	adapter, err := NewAdapterFromConfig(Config{Host: "localhost", Port: 2525, TLSMode: TLSModeNone})
	require.NoError(t, err)

	require.True(t, adapter.IsPermanentSendError(&textproto.Error{Code: 550, Msg: "mailbox unavailable"}))
	require.False(t, adapter.IsPermanentSendError(&textproto.Error{Code: 450, Msg: "mailbox busy"}))
	require.False(t, adapter.IsPermanentSendError(context.DeadlineExceeded))
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

func TestAdapterSend_TLSModeNoneAllowsLoopbackAuth(t *testing.T) {
	for _, authMode := range []string{"plain", authModeLogin} {
		t.Run(authMode, func(t *testing.T) {
			server := startFakeSMTPServer(t, fakeSMTPOptions{supportAuth: true})

			adapter, err := NewAdapterFromConfig(Config{
				Host:     server.host,
				Port:     server.port,
				TLSMode:  TLSModeNone,
				AuthMode: authMode,
				Username: "apikey",
				Password: "secret",
			})
			require.NoError(t, err)

			_, err = adapter.Send(context.Background(), testOutgoingEmail())
			require.NoError(t, err)
			require.Contains(t, server.commands(), "AUTH")
			require.NotContains(t, server.commands(), "STARTTLS")
		})
	}
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
	supportAuth       bool
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

		keepServing, err := handleFakeSMTPCommand(reader, writer, opts, command, upper)
		if err != nil {
			return
		}
		if !keepServing {
			return
		}
	}
}

func handleFakeSMTPCommand(
	reader *bufio.Reader,
	writer *bufio.Writer,
	opts fakeSMTPOptions,
	command string,
	upper string,
) (bool, error) {
	switch upper {
	case "EHLO":
		writeFakeSMTPEHLO(writer, opts)
	case "HELO":
		writeSMTPLine(writer, "250 OK")
	case "STARTTLS":
		writeSMTPLine(writer, "454 TLS unavailable in fake server")
	case "AUTH":
		if err := handleFakeSMTPAuth(reader, writer, command); err != nil {
			return false, err
		}
	case "MAIL", "RCPT":
		writeSMTPLine(writer, "250 OK")
	case "DATA":
		if err := handleFakeSMTPData(reader, writer); err != nil {
			return false, err
		}
	case "QUIT":
		writeSMTPLine(writer, "221 bye")
		return false, nil
	default:
		writeSMTPLine(writer, "250 OK")
	}
	return true, nil
}

func writeFakeSMTPEHLO(writer *bufio.Writer, opts fakeSMTPOptions) {
	writeSMTPLine(writer, "250-fake-smtp")
	if opts.advertiseStartTLS {
		writeSMTPLine(writer, "250-STARTTLS")
	}
	if opts.supportAuth {
		writeSMTPLine(writer, "250-AUTH PLAIN LOGIN")
	}
	writeSMTPLine(writer, "250 OK")
}

func handleFakeSMTPAuth(reader *bufio.Reader, writer *bufio.Writer, command string) error {
	if strings.HasPrefix(strings.ToUpper(command), "AUTH LOGIN") {
		writeSMTPLine(writer, "334 "+base64.StdEncoding.EncodeToString([]byte("Password:")))
		if _, err := reader.ReadString('\n'); err != nil {
			return err
		}
	}
	writeSMTPLine(writer, "235 authenticated")
	return nil
}

func handleFakeSMTPData(reader *bufio.Reader, writer *bufio.Writer) error {
	writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>")
	for {
		dataLine, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(dataLine) == "." {
			break
		}
	}
	writeSMTPLine(writer, "250 queued")
	return nil
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = writer.WriteString(line + "\r\n")
	_ = writer.Flush()
}
