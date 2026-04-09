package mjml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	gomjml "github.com/preslavrachev/gomjml/mjml"
	"github.com/rendis/senda/internal/port"
)

// Compiler implements port.TemplateCompiler using gomjml with an LRU cache.
type Compiler struct {
	cache         *lru.Cache[string, string] // SHA-256 hash → compiled HTML
	publicBaseURL string
}

type Option func(*Compiler)

// WithPublicBaseURL sets the default public base URL used to materialize media
// endpoints during compilation. If empty, video thumbnails fall back to the raw
// thumbnail URL.
func WithPublicBaseURL(baseURL string) Option {
	return func(c *Compiler) {
		c.publicBaseURL = strings.TrimSpace(baseURL)
	}
}

// NewCompiler creates a new MJML compiler with an in-memory LRU cache.
func NewCompiler(opts ...Option) *Compiler {
	cache, _ := lru.New[string, string](1000)
	compiler := &Compiler{cache: cache}
	for _, opt := range opts {
		opt(compiler)
	}
	return compiler
}

// Compile compiles MJML markup into responsive HTML.
// Results are cached by SHA-256 hash of the MJML content.
func (c *Compiler) Compile(ctx context.Context, mjmlContent string) (string, error) {
	if mjmlContent == "" {
		return "", errors.New("mjml: empty input")
	}

	publicBaseURL := c.publicBaseURL
	if ctxBaseURL, ok := port.TemplateCompilerPublicBaseURLFromContext(ctx); ok {
		publicBaseURL = strings.TrimSpace(ctxBaseURL)
	}

	normalizedMJML := rewriteVideoThumbnailMJML(mjmlContent, publicBaseURL)

	h := sha256.Sum256([]byte(normalizedMJML))
	key := hex.EncodeToString(h[:])

	if cached, ok := c.cache.Get(key); ok {
		return cached, nil
	}

	html, err := gomjml.Render(normalizedMJML)
	if err != nil {
		return "", err
	}

	c.cache.Add(key, html)
	return html, nil
}
