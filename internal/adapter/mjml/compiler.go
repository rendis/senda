package mjml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	lru "github.com/hashicorp/golang-lru/v2"
	gomjml "github.com/preslavrachev/gomjml/mjml"
)

// Compiler implements port.TemplateCompiler using gomjml with an LRU cache.
type Compiler struct {
	cache *lru.Cache[string, string] // SHA-256 hash → compiled HTML
}

// NewCompiler creates a new MJML compiler with an in-memory LRU cache.
func NewCompiler() *Compiler {
	cache, _ := lru.New[string, string](1000)
	return &Compiler{cache: cache}
}

// Compile compiles MJML markup into responsive HTML.
// Results are cached by SHA-256 hash of the MJML content.
func (c *Compiler) Compile(_ context.Context, mjmlContent string) (string, error) {
	if mjmlContent == "" {
		return "", errors.New("mjml: empty input")
	}

	h := sha256.Sum256([]byte(mjmlContent))
	key := hex.EncodeToString(h[:])

	if cached, ok := c.cache.Get(key); ok {
		return cached, nil
	}

	html, err := gomjml.Render(mjmlContent)
	if err != nil {
		return "", err
	}

	c.cache.Add(key, html)
	return html, nil
}
