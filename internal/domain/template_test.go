package domain

import "testing"

func TestTemplateType_IsBulk_DefaultsFalse(t *testing.T) {
	tt := TemplateType{Slug: "x"}
	if tt.IsBulk {
		t.Fatalf("zero-value TemplateType.IsBulk must be false")
	}
}
