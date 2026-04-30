package unsubscribe

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

func TestGenerate_Verify_Roundtrip(t *testing.T) {
	key := testKey(t)
	now := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version:          1,
		WorkspaceID:      uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000001"),
		TemplateTypeSlug: "newsletter-mensual",
		TemplateTypeName: "Newsletter Mensual",
		Email:            "juan@ejemplo.com",
		SourceEmailID:    uuid.MustParse("01927e85-aaaa-bbbb-cccc-000000000002"),
		IssuedAt:         now,
		ExpiresAt:        now.Add(365 * 24 * time.Hour),
	}
	tok, err := Generate(p, key)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.Contains(tok, ".") {
		t.Fatalf("token must contain payload.signature separator, got %q", tok)
	}
	got, err := Verify(tok, key, now)
	if err != nil {
		t.Fatalf("Verify roundtrip: %v", err)
	}
	if got.Email != p.Email || got.TemplateTypeSlug != p.TemplateTypeSlug ||
		got.WorkspaceID != p.WorkspaceID || got.SourceEmailID != p.SourceEmailID {
		t.Fatalf("payload mismatch: got=%+v want=%+v", got, p)
	}
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
	key := testKey(t)
	now := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	tok, _ := Generate(p, key)
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	bad := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte("badsig"))
	if _, err := Verify(bad, key, now); err == nil {
		t.Fatal("Verify must reject tampered signature")
	}
}

func TestVerify_RejectsTamperedPayload(t *testing.T) {
	key := testKey(t)
	now := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	tok, _ := Generate(p, key)
	parts := strings.Split(tok, ".")
	bad := base64.RawURLEncoding.EncodeToString([]byte(`{"e":"attacker@x.com","v":1}`)) + "." + parts[1]
	if _, err := Verify(bad, key, now); err == nil {
		t.Fatal("Verify must reject when payload is replaced but signature kept")
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	key := testKey(t)
	issued := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: issued, ExpiresAt: issued.Add(time.Hour),
	}
	tok, _ := Generate(p, key)
	later := issued.Add(2 * time.Hour)
	if _, err := Verify(tok, key, later); err == nil {
		t.Fatal("Verify must reject expired token")
	}
}

func TestVerify_RejectsWrongKey(t *testing.T) {
	key1 := testKey(t)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0xAA
	}
	now := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	tok, _ := Generate(p, key1)
	if _, err := Verify(tok, key2, now); err == nil {
		t.Fatal("Verify must reject token signed with different key")
	}
}

func TestVerify_RejectsUnsupportedVersion(t *testing.T) {
	key := testKey(t)
	now := time.Unix(1729872000, 0).UTC()
	p := Payload{
		Version: 99, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	tok, _ := Generate(p, key)
	if _, err := Verify(tok, key, now); err == nil {
		t.Fatal("Verify must reject unsupported version")
	}
}

func TestGenerate_RejectsWrongKeyLength(t *testing.T) {
	cases := [][]byte{
		nil,
		make([]byte, 0),
		make([]byte, 16),
		make([]byte, 31),
		make([]byte, 33),
		make([]byte, 64),
	}
	now := time.Now().UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	for _, k := range cases {
		if _, err := Generate(p, k); err == nil {
			t.Fatalf("Generate must reject %d-byte key", len(k))
		}
	}
}

func TestVerify_RejectsWrongKeyLength(t *testing.T) {
	valid := testKey(t)
	now := time.Now().UTC()
	p := Payload{
		Version: 1, WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	tok, _ := Generate(p, valid)
	cases := [][]byte{nil, make([]byte, 0), make([]byte, 16), make([]byte, 31), make([]byte, 33), make([]byte, 64)}
	for _, k := range cases {
		if _, err := Verify(tok, k, now); err == nil {
			t.Fatalf("Verify must reject %d-byte key", len(k))
		}
	}
}

func TestVerify_RejectsInvalidBase64Body(t *testing.T) {
	key := testKey(t)
	bad := "!!!not-base64!!!." + base64.RawURLEncoding.EncodeToString([]byte("anysig"))
	if _, err := Verify(bad, key, time.Now().UTC()); err == nil {
		t.Fatal("Verify must reject token with non-base64 payload segment")
	}
}

func TestVerify_RejectsEmptyToken(t *testing.T) {
	key := testKey(t)
	inputs := []string{"", ".", "abc", "abc.", ".abc"}
	for _, in := range inputs {
		if _, err := Verify(in, key, time.Now().UTC()); err == nil {
			t.Fatalf("Verify must reject malformed token %q", in)
		}
	}
}

func TestGenerate_RejectsZeroVersion(t *testing.T) {
	key := testKey(t)
	now := time.Now().UTC()
	p := Payload{
		// Version omitted → zero value
		WorkspaceID: uuid.New(), TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	if _, err := Generate(p, key); err == nil {
		t.Fatal("Generate must reject Version == 0 (no silent default)")
	}
}
