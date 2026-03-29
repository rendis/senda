package mjml_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rendis/senda/internal/adapter/mjml"
	"github.com/rendis/senda/internal/port"
)

func TestCompiler_ImplementsInterface(t *testing.T) {
	var _ port.TemplateCompiler = (*mjml.Compiler)(nil)
}

func TestCompiler_Compile_ValidMJML(t *testing.T) {
	c := mjml.NewCompiler()
	input := `<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>`

	html, err := c.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if html == "" {
		t.Fatal("expected non-empty HTML output")
	}

	if !strings.Contains(html, "Hello") {
		t.Error("expected HTML to contain 'Hello'")
	}

	if !strings.Contains(html, "<html") {
		t.Error("expected HTML to contain '<html' tag")
	}

	if !strings.Contains(html, "<body") {
		t.Error("expected HTML to contain '<body' tag")
	}
}

func TestCompiler_Compile_InvalidMJML(t *testing.T) {
	c := mjml.NewCompiler()
	input := `<not-mjml><broken>`

	_, err := c.Compile(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for invalid MJML")
	}
}

func TestCompiler_Compile_EmptyInput(t *testing.T) {
	c := mjml.NewCompiler()

	_, err := c.Compile(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
