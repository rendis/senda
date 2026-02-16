package port

// Crypto provides symmetric encryption/decryption for sensitive data.
type Crypto interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}
