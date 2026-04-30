package unsubscribe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const supportedVersion = 1

var (
	ErrMalformedToken     = errors.New("unsubscribe: malformed token")
	ErrInvalidSignature   = errors.New("unsubscribe: invalid signature")
	ErrExpired            = errors.New("unsubscribe: token expired")
	ErrUnsupportedVersion = errors.New("unsubscribe: unsupported token version")
)

type Payload struct {
	Version          int       `json:"v"`
	WorkspaceID      uuid.UUID `json:"ws"`
	TemplateTypeID   uuid.UUID `json:"tti"`
	TemplateTypeSlug string    `json:"tt"`
	TemplateTypeName string    `json:"ttn"`
	Email            string    `json:"e"`
	SourceEmailID    uuid.UUID `json:"eid"`
	IssuedAt         time.Time `json:"-"`
	ExpiresAt        time.Time `json:"-"`
}

type wirePayload struct {
	Version          int       `json:"v"`
	WorkspaceID      uuid.UUID `json:"ws"`
	TemplateTypeID   uuid.UUID `json:"tti"`
	TemplateTypeSlug string    `json:"tt"`
	TemplateTypeName string    `json:"ttn"`
	Email            string    `json:"e"`
	SourceEmailID    uuid.UUID `json:"eid"`
	IssuedAt         int64     `json:"iat"`
	ExpiresAt        int64     `json:"exp"`
}

func Generate(p Payload, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("unsubscribe: signing key must be 32 bytes, got %d", len(key))
	}
	if p.Version == 0 {
		return "", fmt.Errorf("unsubscribe: payload version must be set")
	}
	w := wirePayload{
		Version:          p.Version,
		WorkspaceID:      p.WorkspaceID,
		TemplateTypeID:   p.TemplateTypeID,
		TemplateTypeSlug: p.TemplateTypeSlug,
		TemplateTypeName: p.TemplateTypeName,
		Email:            p.Email,
		SourceEmailID:    p.SourceEmailID,
		IssuedAt:         p.IssuedAt.Unix(),
		ExpiresAt:        p.ExpiresAt.Unix(),
	}
	body, err := json.Marshal(&w)
	if err != nil {
		return "", fmt.Errorf("unsubscribe: marshal payload: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func Verify(token string, key []byte, now time.Time) (Payload, error) {
	if len(key) != 32 {
		return Payload{}, fmt.Errorf("unsubscribe: signing key must be 32 bytes, got %d", len(key))
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Payload{}, ErrMalformedToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Payload{}, fmt.Errorf("%w: payload decode: %w", ErrMalformedToken, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Payload{}, fmt.Errorf("%w: signature decode: %w", ErrMalformedToken, err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return Payload{}, ErrInvalidSignature
	}
	var w wirePayload
	if err := json.Unmarshal(body, &w); err != nil {
		return Payload{}, fmt.Errorf("%w: unmarshal: %w", ErrMalformedToken, err)
	}
	if w.Version != supportedVersion {
		return Payload{}, fmt.Errorf("%w: got v%d", ErrUnsupportedVersion, w.Version)
	}
	p := Payload{
		Version:          w.Version,
		WorkspaceID:      w.WorkspaceID,
		TemplateTypeID:   w.TemplateTypeID,
		TemplateTypeSlug: w.TemplateTypeSlug,
		TemplateTypeName: w.TemplateTypeName,
		Email:            w.Email,
		SourceEmailID:    w.SourceEmailID,
		IssuedAt:         time.Unix(w.IssuedAt, 0).UTC(),
		ExpiresAt:        time.Unix(w.ExpiresAt, 0).UTC(),
	}
	if now.After(p.ExpiresAt) {
		return Payload{}, ErrExpired
	}
	return p, nil
}
