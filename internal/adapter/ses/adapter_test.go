package ses

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	sendamime "github.com/rendis/senda/internal/mime"
	"github.com/rendis/senda/internal/port"
)

// --- Mock SES client ---

type mockSESClient struct {
	sendEmailFn func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
	calls       []*sesv2.SendEmailInput
}

func (m *mockSESClient) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	m.calls = append(m.calls, params)
	if m.sendEmailFn != nil {
		return m.sendEmailFn(ctx, params, optFns...)
	}
	return &sesv2.SendEmailOutput{
		MessageId: aws.String("ses-msg-id-abc123"),
	}, nil
}

func (m *mockSESClient) ListEmailIdentities(_ context.Context, _ *sesv2.ListEmailIdentitiesInput, _ ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error) {
	return &sesv2.ListEmailIdentitiesOutput{}, nil
}

func (m *mockSESClient) CreateConfigurationSet(_ context.Context, _ *sesv2.CreateConfigurationSetInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetOutput, error) {
	return &sesv2.CreateConfigurationSetOutput{}, nil
}

func (m *mockSESClient) CreateConfigurationSetEventDestination(_ context.Context, _ *sesv2.CreateConfigurationSetEventDestinationInput, _ ...func(*sesv2.Options)) (*sesv2.CreateConfigurationSetEventDestinationOutput, error) {
	return &sesv2.CreateConfigurationSetEventDestinationOutput{}, nil
}

// --- Tests ---

func TestAdapter_Name(t *testing.T) {
	adapter := NewAdapter(&mockSESClient{}, "us-east-1")
	if got := adapter.Name(); got != "ses" {
		t.Errorf("Name() = %q, want %q", got, "ses")
	}
}

func TestAdapter_HealthCheck(t *testing.T) {
	adapter := NewAdapter(&mockSESClient{}, "us-east-1")
	if err := adapter.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck() error = %v, want nil", err)
	}
}

func TestNewAdapterFromConfig_UsesSESV1ShimForLocalEndpoints(t *testing.T) {
	adapter, err := NewAdapterFromConfig(context.Background(), Config{
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
		EndpointURL:     "http://localhost:4566",
	})
	if err != nil {
		t.Fatalf("NewAdapterFromConfig() error = %v", err)
	}
	if _, ok := adapter.client.(*sesV1API); !ok {
		t.Fatalf("expected SES v1 shim for local endpoint, got %T", adapter.client)
	}
}

func TestNewAdapterFromConfig_UsesSESV2ForAWSDefaultEndpoints(t *testing.T) {
	adapter, err := NewAdapterFromConfig(context.Background(), Config{
		Region:          "us-east-1",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	})
	if err != nil {
		t.Fatalf("NewAdapterFromConfig() error = %v", err)
	}
	if _, ok := adapter.client.(*sesv2.Client); !ok {
		t.Fatalf("expected SES v2 client for default endpoint, got %T", adapter.client)
	}
}

func TestAdapter_ImplementsEmailSender(t *testing.T) {
	var _ port.EmailSender = (*Adapter)(nil)
}

func TestAdapter_Send_Success(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Name: "Acme Support", Address: "support@acme.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Welcome to Acme",
		BodyHTML:   "<html><body><h1>Welcome!</h1></body></html>",
		TrackingID: "trk_test123",
		Headers:    map[string]string{"X-Custom": "test-value"},
	}

	msgID, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if msgID != "ses-msg-id-abc123" {
		t.Errorf("Send() messageID = %q, want %q", msgID, "ses-msg-id-abc123")
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 SES call, got %d", len(mock.calls))
	}

	// Verify raw message was sent.
	rawData := mock.calls[0].Content.Raw.Data
	rawStr := string(rawData)
	if !strings.Contains(rawStr, "From:") {
		t.Error("raw message missing From header")
	}
	if !strings.Contains(rawStr, "user@example.com") {
		t.Error("raw message missing To address")
	}
	if !strings.Contains(rawStr, "Welcome!") {
		t.Error("raw message missing body content")
	}
}

func TestAdapter_Send_WithCC(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		CC:         []port.EmailAddress{{Address: "cc1@example.com"}, {Address: "cc2@example.com"}},
		Subject:    "Test CC",
		BodyHTML:   "<p>Test</p>",
		TrackingID: "trk_cc_test",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rawStr := string(mock.calls[0].Content.Raw.Data)
	if !strings.Contains(rawStr, "cc1@example.com") {
		t.Error("raw message missing CC address 1")
	}
	if !strings.Contains(rawStr, "cc2@example.com") {
		t.Error("raw message missing CC address 2")
	}
}

func TestAdapter_Send_WithReplyTo(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	replyTo := port.EmailAddress{Name: "Reply Here", Address: "reply@example.com"}
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		ReplyTo:    &replyTo,
		Subject:    "Test Reply-To",
		BodyText:   "Plain text body",
		TrackingID: "trk_reply_test",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rawStr := string(mock.calls[0].Content.Raw.Data)
	if !strings.Contains(rawStr, "reply@example.com") {
		t.Error("raw message missing Reply-To address")
	}
}

func TestAdapter_Send_MultipartAlternative(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Multipart",
		BodyHTML:   "<html>HTML content</html>",
		BodyText:   "Plain text content",
		TrackingID: "trk_multipart",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rawStr := string(mock.calls[0].Content.Raw.Data)
	if !strings.Contains(rawStr, "multipart/alternative") {
		t.Error("expected multipart/alternative content type")
	}
	if !strings.Contains(rawStr, "text/plain") {
		t.Error("expected text/plain part")
	}
	if !strings.Contains(rawStr, "text/html") {
		t.Error("expected text/html part")
	}
	if !strings.Contains(rawStr, "HTML content") {
		t.Error("missing HTML content in multipart")
	}
	if !strings.Contains(rawStr, "Plain text content") {
		t.Error("missing plain text content in multipart")
	}
}

func TestAdapter_Send_PlainTextOnly(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Plain",
		BodyText:   "Just text",
		TrackingID: "trk_plain",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	rawStr := string(mock.calls[0].Content.Raw.Data)
	if !strings.Contains(rawStr, "text/plain") {
		t.Error("expected text/plain content type")
	}
	if strings.Contains(rawStr, "multipart") {
		t.Error("should not be multipart for plain text only")
	}
}

func TestAdapter_Send_SESError(t *testing.T) {
	mock := &mockSESClient{
		sendEmailFn: func(_ context.Context, _ *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			return nil, errors.New("ses: message rejected")
		},
	}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Test",
		BodyHTML:   "<p>test</p>",
		TrackingID: "trk_err",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ses: send email") {
		t.Errorf("error = %q, expected to contain 'ses: send email'", err.Error())
	}
}

func TestBuildRawMessage_HTMLOnly(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Name: "Test", Address: "test@example.com"},
		To:         port.EmailAddress{Address: "user@example.com"},
		Subject:    "Test Subject",
		BodyHTML:   "<h1>Hello</h1>",
		TrackingID: "trk_test",
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage() error = %v", err)
	}

	rawStr := string(raw)
	if !strings.Contains(rawStr, "text/html") {
		t.Error("expected text/html content type for HTML-only message")
	}
	if !strings.Contains(rawStr, "Hello") {
		t.Error("missing body content")
	}
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		addr     port.EmailAddress
		contains string
	}{
		{
			name:     "with name",
			addr:     port.EmailAddress{Name: "John Doe", Address: "john@example.com"},
			contains: "john@example.com",
		},
		{
			name:     "without name",
			addr:     port.EmailAddress{Address: "plain@example.com"},
			contains: "plain@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sendamime.FormatAddress(tt.addr)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("FormatAddress() = %q, expected to contain %q", result, tt.contains)
			}
		})
	}
}

func TestDestinationAddresses(t *testing.T) {
	msg := &port.OutgoingEmail{
		To:  port.EmailAddress{Address: "to@example.com"},
		CC:  []port.EmailAddress{{Address: "cc@example.com"}},
		BCC: []port.EmailAddress{{Address: "bcc@example.com"}},
	}

	addrs := destinationAddresses(msg)
	if len(addrs) != 3 {
		t.Fatalf("expected 3 addresses, got %d", len(addrs))
	}
	expected := []string{"to@example.com", "cc@example.com", "bcc@example.com"}
	for i, addr := range addrs {
		if addr != expected[i] {
			t.Errorf("addr[%d] = %q, want %q", i, addr, expected[i])
		}
	}
}

func TestAdapter_Send_WithBCC_IncludesDestination(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		CC:         []port.EmailAddress{{Address: "cc@example.com"}},
		BCC:        []port.EmailAddress{{Address: "bcc1@example.com"}, {Address: "bcc2@example.com"}},
		Subject:    "BCC Test",
		BodyHTML:   "<p>Test</p>",
		TrackingID: "trk_bcc",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}

	dest := mock.calls[0].Destination
	if dest == nil {
		t.Fatal("expected Destination to be set, got nil")
	}

	// To
	if len(dest.ToAddresses) != 1 || dest.ToAddresses[0] != "to@example.com" {
		t.Errorf("ToAddresses = %v, want [to@example.com]", dest.ToAddresses)
	}

	// CC
	if len(dest.CcAddresses) != 1 || dest.CcAddresses[0] != "cc@example.com" {
		t.Errorf("CcAddresses = %v, want [cc@example.com]", dest.CcAddresses)
	}

	// BCC
	if len(dest.BccAddresses) != 2 {
		t.Fatalf("expected 2 BCC addresses, got %d", len(dest.BccAddresses))
	}
	if dest.BccAddresses[0] != "bcc1@example.com" || dest.BccAddresses[1] != "bcc2@example.com" {
		t.Errorf("BccAddresses = %v, want [bcc1@example.com bcc2@example.com]", dest.BccAddresses)
	}
}

func TestAdapter_Send_NoBCC_DestinationStillSet(t *testing.T) {
	mock := &mockSESClient{}
	adapter := NewAdapter(mock, "us-east-1")

	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "No BCC",
		BodyHTML:   "<p>Test</p>",
		TrackingID: "trk_nobcc",
	}

	_, err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	dest := mock.calls[0].Destination
	if dest == nil {
		t.Fatal("expected Destination to be set even without BCC")
	}
	if len(dest.ToAddresses) != 1 {
		t.Errorf("expected 1 To address, got %d", len(dest.ToAddresses))
	}
	if dest.CcAddresses != nil {
		t.Errorf("expected nil CcAddresses, got %v", dest.CcAddresses)
	}
	if dest.BccAddresses != nil {
		t.Errorf("expected nil BccAddresses, got %v", dest.BccAddresses)
	}
}

func TestBuildRawMessage_HeaderInjection_ValueSanitized(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Header Injection Test",
		BodyHTML:   "<p>test</p>",
		TrackingID: "trk_inj",
		Headers: map[string]string{
			"X-Custom": "safe-value\r\nBcc: attacker@evil.com",
		},
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage() error = %v", err)
	}

	rawStr := string(raw)
	// The CRLF should be stripped, so the injected header is collapsed into the value.
	// This prevents the attacker from injecting a new header line.
	if strings.Contains(rawStr, "safe-value\r\n") {
		t.Error("header injection: CRLF should be stripped from header value")
	}
	// The value should appear as one continuous string without line breaks.
	if !strings.Contains(rawStr, "X-Custom: safe-valueBcc: attacker@evil.com") {
		t.Error("expected sanitized value with CRLF stripped (value concatenated into single header line)")
	}
	// Crucially, "Bcc:" should NOT appear as a separate header line.
	// Count how many times "Bcc:" appears as a header start (after a newline).
	lines := strings.Split(rawStr, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Bcc:") {
			t.Error("header injection: 'Bcc:' should not appear as a standalone header line")
		}
	}
}

func TestBuildRawMessage_HeaderInjection_UnsafeKeySkipped(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Key Injection Test",
		BodyHTML:   "<p>test</p>",
		TrackingID: "trk_key_inj",
		Headers: map[string]string{
			"X-Safe-Key":           "good",
			"X-Bad Key With Space": "bad1",
			"X-Bad\r\nKey":         "bad2",
			"X-Bad:Key":            "bad3",
		},
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage() error = %v", err)
	}

	rawStr := string(raw)
	if !strings.Contains(rawStr, "X-Safe-Key: good") {
		t.Error("expected safe header key to be included")
	}
	if strings.Contains(rawStr, "bad1") {
		t.Error("header with space in key should be skipped")
	}
	if strings.Contains(rawStr, "bad2") {
		t.Error("header with CRLF in key should be skipped")
	}
	if strings.Contains(rawStr, "bad3") {
		t.Error("header with colon in key should be skipped")
	}
}

func TestBuildRawMessage_ContentTransferEncoding_8bit(t *testing.T) {
	msg := &port.OutgoingEmail{
		From:       port.EmailAddress{Address: "from@example.com"},
		To:         port.EmailAddress{Address: "to@example.com"},
		Subject:    "Encoding Test",
		BodyHTML:   "<p>HTML</p>",
		BodyText:   "Plain text",
		TrackingID: "trk_encoding",
	}

	raw, err := sendamime.BuildRawMessage(msg)
	if err != nil {
		t.Fatalf("BuildRawMessage() error = %v", err)
	}

	rawStr := string(raw)
	if strings.Contains(rawStr, "quoted-printable") {
		t.Error("should use 8bit, not quoted-printable")
	}
	if !strings.Contains(rawStr, "Content-Transfer-Encoding: 8bit") {
		t.Error("expected Content-Transfer-Encoding: 8bit")
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal value", "normal value"},
		{"with\rnewline", "withnewline"},
		{"with\nnewline", "withnewline"},
		{"with\r\ncrlf", "withcrlf"},
		{"clean", "clean"},
		{"multi\r\n\r\ninjection", "multiinjection"},
	}

	for _, tt := range tests {
		got := sendamime.SanitizeHeaderValue(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
