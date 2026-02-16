package dkim_test

import (
	"bytes"
	"strings"
	"testing"

	godkim "github.com/emersion/go-msgauth/dkim"
	"github.com/senda-app/senda/internal/adapter/dkim"
)

const testEmail = "From: sender@example.com\r\n" +
	"To: recipient@example.com\r\n" +
	"Subject: Test\r\n" +
	"Date: Mon, 01 Jan 2024 00:00:00 +0000\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\n" +
	"Hello, World!\r\n"

func TestSign_RoundTrip(t *testing.T) {
	privPEM, pubB64, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	signed, err := dkim.Sign([]byte(testEmail), "example.com", "test", privPEM)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	if !strings.Contains(string(signed), "DKIM-Signature:") {
		t.Fatal("signed message does not contain DKIM-Signature header")
	}

	// Verify the signature using go-msgauth's verify with custom DNS lookup
	txtValue := dkim.DNSTXTValue(pubB64)
	verifications, err := godkim.VerifyWithOptions(bytes.NewReader(signed), &godkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) {
			return []string{txtValue}, nil
		},
	})
	if err != nil {
		t.Fatalf("failed to verify: %v", err)
	}

	if len(verifications) == 0 {
		t.Fatal("expected at least one verification result")
	}

	for _, v := range verifications {
		if v.Err != nil {
			t.Errorf("verification failed for domain %s: %v", v.Domain, v.Err)
		}
	}
}

func TestSign_InvalidPrivateKey(t *testing.T) {
	_, err := dkim.Sign([]byte(testEmail), "example.com", "test", []byte("not-a-key"))
	if err == nil {
		t.Fatal("expected error for invalid private key")
	}
}

func TestSign_EmptyMessage(t *testing.T) {
	privPEM, _, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	_, err = dkim.Sign(nil, "example.com", "test", privPEM)
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestSign_EmptyDomain(t *testing.T) {
	privPEM, _, _ := dkim.GenerateKeyPair()
	rawMsg := []byte("From: test@example.com\r\n\r\nHello")
	_, err := dkim.Sign(rawMsg, "", "selector", privPEM)
	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestSign_EmptySelector(t *testing.T) {
	privPEM, _, _ := dkim.GenerateKeyPair()
	rawMsg := []byte("From: test@example.com\r\n\r\nHello")
	_, err := dkim.Sign(rawMsg, "example.com", "", privPEM)
	if err == nil {
		t.Fatal("expected error for empty selector")
	}
}
