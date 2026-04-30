package unsubscribe

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPeekWorkspaceID_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	ws := uuid.MustParse("01927e80-aaaa-bbbb-cccc-000000000001")
	p := Payload{
		Version: 1, WorkspaceID: ws, TemplateTypeSlug: "x", TemplateTypeName: "X",
		Email: "a@b.com", SourceEmailID: uuid.New(), IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	tok, err := Generate(p, key)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got, ok := PeekWorkspaceID(tok)
	if !ok || got != ws {
		t.Fatalf("PeekWorkspaceID got %v ok=%v want %v", got, ok, ws)
	}
}

func TestPeekWorkspaceID_Malformed(t *testing.T) {
	cases := []string{"", ".", "abc", "abc.", "!!!.def"}
	for _, c := range cases {
		if _, ok := PeekWorkspaceID(c); ok {
			t.Fatalf("PeekWorkspaceID(%q) must reject malformed token", c)
		}
	}
}
