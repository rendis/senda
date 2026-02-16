package dkim

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
)

// GenerateKeyPair generates an RSA-2048 key pair for DKIM signing.
// It returns the private key in PEM format and the public key as base64-encoded
// DER/PKIX format suitable for DNS TXT records.
func GenerateKeyPair() (privateKeyPEM []byte, publicKeyBase64 string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, "", err
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})

	pubDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, "", err
	}

	pubB64 := base64.StdEncoding.EncodeToString(pubDER)

	return privPEM, pubB64, nil
}
