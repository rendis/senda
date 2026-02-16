package tracking

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	prefix    = "snd_"
	randBytes = 14 // 14 bytes = 28 hex chars; total = 4 + 28 = 32 (fits VARCHAR(32))
)

// NewTrackingID generates a new tracking ID with format "snd_" + 28 hex chars (32 total).
func NewTrackingID() string {
	b := make([]byte, randBytes)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return prefix + hex.EncodeToString(b)
}
