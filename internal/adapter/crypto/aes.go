package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	minMasterKeyLen = 32
	nonceSize       = 12
	hkdfInfo        = "senda-aes-256-gcm"
	aes256KeyLen    = 32

	// hkdfSalt is a static deployment-wide salt for HKDF key derivation.
	// Changing this value requires re-encrypting ALL existing encrypted data
	// (adapter credentials). Cryptographic isolation between
	// environments is provided by the per-deployment master key, not this salt.
	hkdfSalt = "senda-v1"
)

// AESCrypto implements the port.Crypto interface using AES-256-GCM
// with HKDF-derived keys.
type AESCrypto struct {
	gcm cipher.AEAD
}

// NewAESCrypto creates a new AESCrypto instance. The masterKey must be at
// least 32 characters. A 32-byte AES key is derived from masterKey using
// HKDF (SHA-256).
func NewAESCrypto(masterKey string) (*AESCrypto, error) {
	if len(masterKey) < minMasterKeyLen {
		return nil, fmt.Errorf("master key must be at least %d characters, got %d", minMasterKeyLen, len(masterKey))
	}

	derivedKey, err := deriveKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("deriving AES key: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	return &AESCrypto{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The returned ciphertext format is: nonce (12 bytes) || ciphertext || GCM tag.
func (a *AESCrypto) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	// Seal appends the ciphertext+tag to nonce, so result is nonce||ct||tag.
	return a.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt. It expects the format:
// nonce (12 bytes) || ciphertext || GCM tag.
func (a *AESCrypto) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	plaintext, err := a.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

// deriveKey uses HKDF (SHA-256) to derive a 32-byte AES-256 key from the
// master key string.
func deriveKey(masterKey string) ([]byte, error) {
	r := hkdf.New(sha256.New, []byte(masterKey), []byte(hkdfSalt), []byte(hkdfInfo))

	key := make([]byte, aes256KeyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("reading derived key: %w", err)
	}

	return key, nil
}
