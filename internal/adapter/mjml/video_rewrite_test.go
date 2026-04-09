package mjml_test

import (
	"context"
	"strings"
	"testing"

	"github.com/rendis/senda/internal/adapter/mjml"
	"github.com/rendis/senda/internal/port"
)

const videoBlockMJML = `<mjml><mj-body><mj-section><mj-column><mj-image src="https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ" css-class="senda-video" /></mj-column></mj-section></mj-body></mjml>`

func TestCompiler_Compile_RewritesVideoThumbnailWithConfiguredPublicBaseURL(t *testing.T) {
	c := mjml.NewCompiler(mjml.WithPublicBaseURL("https://cdn.example.com"))

	html, err := c.Compile(context.Background(), videoBlockMJML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "https://cdn.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg") {
		t.Fatalf("expected compiled html to contain rewritten thumbnail URL, got %q", html)
	}
}

func TestCompiler_Compile_ContextBaseURLOverridesConfiguredPublicBaseURL(t *testing.T) {
	c := mjml.NewCompiler(mjml.WithPublicBaseURL("https://cdn.example.com"))
	ctx := port.WithTemplateCompilerPublicBaseURL(context.Background(), "https://preview.example.com")

	html, err := c.Compile(ctx, videoBlockMJML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "https://preview.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg") {
		t.Fatalf("expected compiled html to contain request-scoped thumbnail URL, got %q", html)
	}
	if strings.Contains(html, "https://cdn.example.com/public/video-thumbnail") {
		t.Fatalf("expected configured base URL to be overridden, got %q", html)
	}
}

func TestCompiler_Compile_RepairsLegacyWrappedVideoThumbnailURLs(t *testing.T) {
	c := mjml.NewCompiler(mjml.WithPublicBaseURL("https://cdn.example.com"))
	input := `<mjml><mj-body><mj-section><mj-column><mj-image src="http://localhost:8081/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ" css-class="senda-video" /></mj-column></mj-section></mj-body></mjml>`

	html, err := c.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(html, "https://cdn.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg") {
		t.Fatalf("expected compiled html to repair legacy wrapped URL, got %q", html)
	}
	if strings.Contains(html, "http://localhost:8081/public/video-thumbnail") {
		t.Fatalf("expected legacy localhost wrapper to be removed, got %q", html)
	}
}
