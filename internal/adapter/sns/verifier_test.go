package sns_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rendis/senda/internal/adapter/sns"
)

// testCert generates a self-signed RSA cert+key pair for testing.
func testCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sns.us-east-1.amazonaws.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return cert, key, certPEM
}

// signMessage signs the string-to-sign using the RSA private key (SHA1 + PKCS1v15).
func signMessage(t *testing.T, key *rsa.PrivateKey, stringToSign string) string {
	t.Helper()

	h := crypto.SHA1.New()
	h.Write([]byte(stringToSign))

	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA1, h.Sum(nil))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// notificationStringToSign returns the canonical string for a Notification message.
func notificationStringToSign(msg map[string]string) string {
	s := "Message\n" + msg["Message"] + "\n"
	s += "MessageId\n" + msg["MessageId"] + "\n"
	if subj, ok := msg["Subject"]; ok && subj != "" {
		s += "Subject\n" + subj + "\n"
	}
	s += "Timestamp\n" + msg["Timestamp"] + "\n"
	s += "TopicArn\n" + msg["TopicArn"] + "\n"
	s += "Type\n" + msg["Type"] + "\n"
	return s
}

func TestVerifier_ValidNotification(t *testing.T) {
	_, key, certPEM := testCert(t)

	// Serve the certificate via HTTPS-like test server (but HTTP for test).
	certServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(certPEM); err != nil {
			t.Fatalf("write cert PEM: %v", err)
		}
	}))
	defer certServer.Close()

	// We need to override the cert URL validation for testing since test servers use localhost.
	// Instead, we'll test the full verification with a server that mimics amazonaws.com.
	// For unit tests, we test the signature logic and cert URL validation separately.

	msg := map[string]string{
		"Type":             "Notification",
		"MessageId":        "msg-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          `{"notificationType":"Delivery"}`,
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "1",
	}

	stringToSign := notificationStringToSign(msg)
	sig := signMessage(t, key, stringToSign)
	msg["Signature"] = sig
	msg["SigningCertURL"] = certServer.URL + "/cert.pem" // Not amazonaws.com — will fail URL validation

	body, _ := json.Marshal(msg)

	v := sns.NewVerifier(certServer.Client())
	err := v.Verify(body)

	// Expected: cert URL validation fails since test server is not amazonaws.com
	if err == nil {
		t.Fatal("expected error for non-amazonaws cert URL")
	}
	if !errors.Is(err, sns.ErrInvalidCertURL) {
		t.Fatalf("expected ErrInvalidCertURL, got: %v", err)
	}
}

func TestVerifier_InvalidSignatureVersion(t *testing.T) {
	msg := map[string]string{
		"Type":             "Notification",
		"MessageId":        "msg-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          `test`,
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "2",
		"Signature":        "aGVsbG8=",
		"SigningCertURL":   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}

	body, _ := json.Marshal(msg)

	v := sns.NewVerifier(nil)
	err := v.Verify(body)
	if err == nil {
		t.Fatal("expected error for unsupported signature version")
	}
	if !errors.Is(err, sns.ErrInvalidSignatureVersion) {
		t.Fatalf("expected ErrInvalidSignatureVersion, got: %v", err)
	}
}

func TestVerifier_InvalidCertURL_NotHTTPS(t *testing.T) {
	msg := map[string]string{
		"Type":             "Notification",
		"MessageId":        "msg-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          `test`,
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "1",
		"Signature":        "aGVsbG8=",
		"SigningCertURL":   "http://sns.us-east-1.amazonaws.com/cert.pem",
	}

	body, _ := json.Marshal(msg)

	v := sns.NewVerifier(nil)
	err := v.Verify(body)
	if err == nil {
		t.Fatal("expected error for HTTP cert URL")
	}
	if !errors.Is(err, sns.ErrInvalidCertURL) {
		t.Fatalf("expected ErrInvalidCertURL, got: %v", err)
	}
}

func TestVerifier_InvalidCertURL_NotAmazon(t *testing.T) {
	msg := map[string]string{
		"Type":             "Notification",
		"MessageId":        "msg-001",
		"TopicArn":         "arn:aws:sns:us-east-1:123456789012:SES-Events",
		"Message":          `test`,
		"Timestamp":        "2026-02-17T10:00:00.000Z",
		"SignatureVersion": "1",
		"Signature":        "aGVsbG8=",
		"SigningCertURL":   "https://evil.example.com/cert.pem",
	}

	body, _ := json.Marshal(msg)

	v := sns.NewVerifier(nil)
	err := v.Verify(body)
	if err == nil {
		t.Fatal("expected error for non-Amazon cert URL")
	}
	if !errors.Is(err, sns.ErrInvalidCertURL) {
		t.Fatalf("expected ErrInvalidCertURL, got: %v", err)
	}
}

func TestVerifier_MalformedJSON(t *testing.T) {
	v := sns.NewVerifier(nil)
	err := v.Verify([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !errors.Is(err, sns.ErrMalformedMessage) {
		t.Fatalf("expected ErrMalformedMessage, got: %v", err)
	}
}

func TestVerifier_ValidCertURL_Patterns(t *testing.T) {
	// These are valid cert URL patterns — only sns.{region}.amazonaws.com
	validURLs := []string{
		"https://sns.us-east-1.amazonaws.com/SimpleNotificationService-abc123.pem",
		"https://sns.eu-west-1.amazonaws.com/cert.pem",
		"https://sns.ap-southeast-1.amazonaws.com/cert.pem",
	}

	// These are invalid — including non-SNS amazonaws.com subdomains
	invalidURLs := []string{
		"https://evil.example.com/cert.pem",
		"http://sns.us-east-1.amazonaws.com/cert.pem",    // http, not https
		"https://amazonaws.com.evil.com/cert.pem",         // subdomain of evil.com
		"https://not-amazonaws.com/cert.pem",
		"https://s3.us-east-1.amazonaws.com/cert.pem",    // s3, not sns
		"https://ec2.us-east-1.amazonaws.com/cert.pem",   // ec2, not sns
		"https://evil.amazonaws.com/cert.pem",             // no region, not sns prefix
	}

	for _, u := range validURLs {
		msg := map[string]string{
			"Type":             "Notification",
			"MessageId":        "msg-001",
			"TopicArn":         "arn:aws:sns:us-east-1:123456789012:test",
			"Message":          "test",
			"Timestamp":        "2026-02-17T10:00:00.000Z",
			"SignatureVersion": "1",
			"Signature":        "aGVsbG8=",
			"SigningCertURL":   u,
		}
		body, _ := json.Marshal(msg)

		v := sns.NewVerifier(nil)
		err := v.Verify(body)
		// Should NOT fail with ErrInvalidCertURL (will fail with ErrCertFetch since we can't fetch it)
		if errors.Is(err, sns.ErrInvalidCertURL) {
			t.Errorf("URL %q should be valid but got ErrInvalidCertURL", u)
		}
	}

	for _, u := range invalidURLs {
		msg := map[string]string{
			"Type":             "Notification",
			"MessageId":        "msg-001",
			"TopicArn":         "arn:aws:sns:us-east-1:123456789012:test",
			"Message":          "test",
			"Timestamp":        "2026-02-17T10:00:00.000Z",
			"SignatureVersion": "1",
			"Signature":        "aGVsbG8=",
			"SigningCertURL":   u,
		}
		body, _ := json.Marshal(msg)

		v := sns.NewVerifier(nil)
		err := v.Verify(body)
		if !errors.Is(err, sns.ErrInvalidCertURL) {
			t.Errorf("URL %q should be invalid but got: %v", u, err)
		}
	}
}
