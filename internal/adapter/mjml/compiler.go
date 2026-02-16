package mjml

import (
	"context"
	"errors"

	gomjml "github.com/preslavrachev/gomjml/mjml"
)

// Compiler implements port.TemplateCompiler using gomjml.
type Compiler struct{}

// NewCompiler creates a new MJML compiler.
func NewCompiler() *Compiler {
	return &Compiler{}
}

// Compile compiles MJML markup into responsive HTML.
func (c *Compiler) Compile(_ context.Context, mjmlContent string) (string, error) {
	if mjmlContent == "" {
		return "", errors.New("mjml: empty input")
	}

	html, err := gomjml.Render(mjmlContent)
	if err != nil {
		return "", err
	}

	return html, nil
}
