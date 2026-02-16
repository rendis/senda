package dkim

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"

	godkim "github.com/emersion/go-msgauth/dkim"
)

// Sign adds a DKIM-Signature header to a raw email message.
// It takes the private key as a PEM-encoded parameter, making it a standalone
// function with no store or crypto service dependencies.
func Sign(rawMsg []byte, domain string, selector string, privateKeyPEM []byte) ([]byte, error) {
	if len(rawMsg) == 0 {
		return nil, errors.New("dkim: empty message")
	}
	if domain == "" {
		return nil, errors.New("dkim: empty domain")
	}
	if selector == "" {
		return nil, errors.New("dkim: empty selector")
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, errors.New("dkim: invalid PEM data")
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("dkim: failed to parse private key: " + err.Error())
	}

	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, errors.New("dkim: private key does not implement crypto.Signer")
	}

	opts := &godkim.SignOptions{
		Domain:     domain,
		Selector:   selector,
		Signer:     signer,
		Hash:       crypto.SHA256,
		HeaderKeys: []string{"From", "To", "Subject", "Date", "MIME-Version", "Content-Type"},
	}

	r := bytes.NewReader(rawMsg)
	var signed bytes.Buffer

	if err := godkim.Sign(&signed, r, opts); err != nil {
		return nil, err
	}

	return signed.Bytes(), nil
}
