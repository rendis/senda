package service_test

import (
	"testing"

	"github.com/rendis/senda/internal/service"
)

func TestVariableRenderer_Render_EventVars(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.Render(
		"Hello {{ event.name }}, your order #{{ event.order_id }} is ready",
		nil,
		map[string]any{"name": "Alice", "order_id": 42},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hello Alice, your order #42 is ready"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_InjectorVars(t *testing.T) {
	r := service.NewVariableRenderer()

	injectors := map[string]map[string]any{
		"brand": {"logo_url": "https://example.com/logo.png", "name": "Acme"},
	}

	result, err := r.Render(
		"Welcome to {{ injector.brand.name }}! Logo: {{ injector.brand.logo_url }}",
		injectors,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Welcome to Acme! Logo: https://example.com/logo.png"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_Mixed(t *testing.T) {
	r := service.NewVariableRenderer()

	injectors := map[string]map[string]any{
		"company": {"support_email": "help@acme.com"},
	}
	eventVars := map[string]any{
		"user_name": "Bob",
	}

	result, err := r.Render(
		"Hi {{ event.user_name }}, contact us at {{ injector.company.support_email }}",
		injectors,
		eventVars,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Hi Bob, contact us at help@acme.com"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_NoVars(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.Render("No variables here", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No variables here" {
		t.Fatalf("expected unchanged string, got %q", result)
	}
}

func TestVariableRenderer_Render_MissingEventVar(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.Render(
		"Hello {{ event.missing }}",
		nil,
		map[string]any{"name": "Alice"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Missing variables should render as empty string
	if result != "Hello " {
		t.Fatalf("expected 'Hello ', got %q", result)
	}
}

func TestVariableRenderer_Render_MissingInjectorField(t *testing.T) {
	r := service.NewVariableRenderer()

	injectors := map[string]map[string]any{
		"brand": {"name": "Acme"},
	}

	result, err := r.Render(
		"Logo: {{ injector.brand.missing_field }}",
		injectors,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Logo: " {
		t.Fatalf("expected 'Logo: ', got %q", result)
	}
}

func TestVariableRenderer_Render_MissingInjectorName(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.Render(
		"Val: {{ injector.nonexistent.field }}",
		map[string]map[string]any{},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Val: " {
		t.Fatalf("expected 'Val: ', got %q", result)
	}
}

func TestVariableRenderer_Render_SpacingVariations(t *testing.T) {
	r := service.NewVariableRenderer()

	// Should handle variable spacing in braces
	result, err := r.Render(
		"A={{event.a}} B={{ event.b }} C={{  event.c  }}",
		nil,
		map[string]any{"a": "1", "b": "2", "c": "3"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "A=1 B=2 C=3"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_NumericAndBoolValues(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.Render(
		"Count: {{ event.count }}, Active: {{ event.active }}",
		nil,
		map[string]any{"count": 123, "active": true},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Count: 123, Active: true"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_NilInjectorValue(t *testing.T) {
	r := service.NewVariableRenderer()

	injectors := map[string]map[string]any{
		"brand": {"name": nil},
	}

	result, err := r.Render(
		"Brand: {{ injector.brand.name }}",
		injectors,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Brand: " {
		t.Fatalf("expected 'Brand: ', got %q", result)
	}
}

func TestVariableRenderer_RenderHTML_EscapesXSS(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.RenderHTML(
		"<p>Hello {{ event.name }}</p>",
		nil,
		map[string]any{"name": `<script>alert("xss")</script>`},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "<p>Hello &lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;</p>"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_RenderHTML_EscapesAmpersand(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.RenderHTML(
		"<a href=\"{{ event.url }}\">Link</a>",
		nil,
		map[string]any{"url": "https://example.com?a=1&b=2"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<a href="https://example.com?a=1&amp;b=2">Link</a>`
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_RenderHTML_EscapesInjectorValues(t *testing.T) {
	r := service.NewVariableRenderer()

	injectors := map[string]map[string]any{
		"brand": {"tagline": `"Best & Greatest" <em>Company</em>`},
	}

	result, err := r.RenderHTML(
		"<div>{{ injector.brand.tagline }}</div>",
		injectors,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := `<div>&#34;Best &amp; Greatest&#34; &lt;em&gt;Company&lt;/em&gt;</div>`
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_RenderHTML_SafeValuesUnchanged(t *testing.T) {
	r := service.NewVariableRenderer()

	result, err := r.RenderHTML(
		"<p>{{ event.name }}</p>",
		nil,
		map[string]any{"name": "Alice"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "<p>Alice</p>"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestVariableRenderer_Render_PlainText_NoEscaping(t *testing.T) {
	r := service.NewVariableRenderer()

	// Plain text Render should NOT escape HTML entities
	result, err := r.Render(
		"Hello {{ event.name }}",
		nil,
		map[string]any{"name": `<b>Alice & Bob</b>`},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Hello <b>Alice & Bob</b>"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestRender_SystemPrefix(t *testing.T) {
	r := service.NewVariableRenderer()
	body := `Hi {{ event.name }}, manage at {{ system.preferences_url }} or unsubscribe at {{ system.unsubscribe_url }}.`
	injectors := map[string]map[string]any{}
	eventVars := map[string]any{"name": "Juan"}
	systemVars := map[string]string{
		"unsubscribe_url": "https://x.test/api/v1/u/abc",
		"preferences_url": "https://x.test/u/abc/preferences",
	}
	got, err := r.RenderWithSystem(body, injectors, eventVars, systemVars)
	if err != nil {
		t.Fatalf("RenderWithSystem: %v", err)
	}
	want := `Hi Juan, manage at https://x.test/u/abc/preferences or unsubscribe at https://x.test/api/v1/u/abc.`
	if got != want {
		t.Fatalf("\nwant: %s\ngot:  %s", want, got)
	}
}

func TestRender_SystemPrefix_MissingKeyResolvesEmpty(t *testing.T) {
	r := service.NewVariableRenderer()
	body := `[{{ system.unknown }}]`
	out, err := r.RenderWithSystem(body, nil, nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != `[]` {
		t.Fatalf("expected empty replacement, got %q", out)
	}
}

func TestRender_SystemPrefix_DelegatedFromRender(t *testing.T) {
	// Verify that the legacy Render(body, injectors, eventVars) signature still works
	// and returns no system substitutions when systemVars are not supplied.
	r := service.NewVariableRenderer()
	body := `[{{ system.unsubscribe_url }}]`
	out, err := r.Render(body, nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != `[]` {
		t.Fatalf("legacy Render must resolve system vars to empty when none supplied, got %q", out)
	}
}

func TestRender_SystemPrefix_PreservesAmpersandsInURLs(t *testing.T) {
	r := service.NewVariableRenderer()
	body := `Click {{ system.unsubscribe_url }}`
	systemVars := map[string]string{
		"unsubscribe_url": "https://example.com/u/abc?token=xyz&signature=def&exp=1",
	}
	out, err := r.RenderWithSystem(body, nil, nil, systemVars)
	if err != nil {
		t.Fatalf("RenderWithSystem: %v", err)
	}
	want := `Click https://example.com/u/abc?token=xyz&signature=def&exp=1`
	if out != want {
		t.Fatalf("system var URL must be passed through unescaped\n want: %q\n got:  %q", want, out)
	}
}
