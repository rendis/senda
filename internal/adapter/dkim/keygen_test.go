package dkim_test

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"github.com/senda-app/senda/internal/adapter/dkim"
)

func TestGenerateKeyPair_ReturnsNonEmpty(t *testing.T) {
	privPEM, pubB64, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(privPEM) == 0 {
		t.Fatal("expected non-empty private key PEM")
	}

	if pubB64 == "" {
		t.Fatal("expected non-empty public key base64")
	}
}

func TestGenerateKeyPair_PrivatePEMIsValid(t *testing.T) {
	privPEM, _, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	block, _ := pem.Decode(privPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}

	if block.Type != "PRIVATE KEY" {
		t.Fatalf("expected PEM type 'PRIVATE KEY', got %q", block.Type)
	}

	_, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
}

func TestGenerateKeyPair_PublicKeyBase64IsValid(t *testing.T) {
	_, pubB64, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pubDER, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	_, err = x509.ParsePKIXPublicKey(pubDER)
	if err != nil {
		t.Fatalf("failed to parse public key: %v", err)
	}
}

func TestGenerateKeyPair_ProducesDifferentKeys(t *testing.T) {
	priv1, pub1, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	priv2, pub2, err := dkim.GenerateKeyPair()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(priv1) == string(priv2) {
		t.Error("expected different private keys")
	}

	if pub1 == pub2 {
		t.Error("expected different public keys")
	}
}
