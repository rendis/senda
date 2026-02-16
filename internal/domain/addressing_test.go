package domain

import (
	"errors"
	"testing"
)

func TestParseRef_Valid(t *testing.T) {
	ref, err := ParseRef("latam:acme:welcome")
	if err != nil {
		t.Fatalf("ParseRef(\"latam:acme:welcome\") error: %v", err)
	}
	if ref.TenantCode != "latam" {
		t.Errorf("TenantCode = %q, want \"latam\"", ref.TenantCode)
	}
	if ref.WorkspaceCode != "acme" {
		t.Errorf("WorkspaceCode = %q, want \"acme\"", ref.WorkspaceCode)
	}
	if ref.TemplateType != "welcome" {
		t.Errorf("TemplateType = %q, want \"welcome\"", ref.TemplateType)
	}
}

func TestParseRef_WithColonsInTemplateType(t *testing.T) {
	// SplitN with 3 means the third part captures everything after second colon
	ref, err := ParseRef("latam:acme:welcome:extra")
	if err != nil {
		t.Fatalf("ParseRef error: %v", err)
	}
	if ref.TemplateType != "welcome:extra" {
		t.Errorf("TemplateType = %q, want \"welcome:extra\"", ref.TemplateType)
	}
}

func TestParseRef_Invalid(t *testing.T) {
	cases := []string{
		"",
		"only-one",
		"two:parts",
		"::",
		"a::b",
		"::c",
	}
	for _, c := range cases {
		_, err := ParseRef(c)
		if err == nil {
			t.Errorf("ParseRef(%q) should return error", c)
		}
		if !errors.Is(err, ErrInvalidRef) {
			t.Errorf("ParseRef(%q) error = %v, want ErrInvalidRef", c, err)
		}
	}
}
