package service_test

import (
	"testing"

	"github.com/rendis/senda/internal/service"
)

func TestRenderStaticInjectorPreview_ReplacesKnownStaticInjectorsOnly(t *testing.T) {
	input := `<mj-text>{{ injector.brand.name }} {{ injector.student.name }} {{ event.user_name }}</mj-text>`

	got := service.RenderStaticInjectorPreview(input, map[string]map[string]any{
		"brand": {"name": "Acme"},
	})

	want := `<mj-text>Acme {{ injector.student.name }} {{ event.user_name }}</mj-text>`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderStaticInjectorPreview_RendersHTMLVerbatim(t *testing.T) {
	input := `<mj-text>{{ injector.legal.footer_html }}</mj-text>`

	got := service.RenderStaticInjectorPreview(input, map[string]map[string]any{
		"legal": {"footer_html": "<strong>Footer</strong>"},
	})

	want := `<mj-text><strong>Footer</strong></mj-text>`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
