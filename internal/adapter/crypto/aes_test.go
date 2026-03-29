package crypto_test

import (
	"bytes"
	"testing"

	"github.com/rendis/senda/internal/adapter/crypto"
)

const validMasterKey = "this-is-a-valid-master-key-32chars!"

func TestNewAESCrypto_ValidKey(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil AESCrypto")
	}
}

func TestNewAESCrypto_ShortKey(t *testing.T) {
	_, err := crypto.NewAESCrypto("short")
	if err == nil {
		t.Fatal("expected error for short master key")
	}
}

func TestNewAESCrypto_EmptyKey(t *testing.T) {
	_, err := crypto.NewAESCrypto("")
	if err == nil {
		t.Fatal("expected error for empty master key")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte("adapter credentials secret value")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_DifferentCiphertextForSamePlaintext(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte("same plaintext twice")
	ct1, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("first encrypt error: %v", err)
	}

	ct2, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("second encrypt error: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Fatal("expected different ciphertexts for same plaintext (random nonce)")
	}
}

func TestDecrypt_CorruptedCiphertext(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte("some secret data")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	// Flip a byte in the ciphertext body (after nonce)
	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = c.Decrypt(corrupted)
	if err == nil {
		t.Fatal("expected error for corrupted ciphertext")
	}
}

func TestDecrypt_TruncatedCiphertext(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Less than 12 bytes (nonce size)
	_, err = c.Decrypt([]byte("short"))
	if err == nil {
		t.Fatal("expected error for truncated ciphertext")
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte{}
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("empty round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_LargePlaintext(t *testing.T) {
	c, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 MB of data
	plaintext := make([]byte, 1<<20)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("large round-trip mismatch: lengths got=%d want=%d", len(decrypted), len(plaintext))
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	c1, err := crypto.NewAESCrypto(validMasterKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c2, err := crypto.NewAESCrypto("another-valid-master-key-32chars!!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte("secret data for key isolation test")
	ciphertext, err := c1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	_, err = c2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}
