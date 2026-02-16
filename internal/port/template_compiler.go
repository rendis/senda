package port

import "context"

// TemplateCompiler compiles MJML templates into HTML.
type TemplateCompiler interface {
	// Compile takes MJML source with resolved variables and returns HTML.
	Compile(ctx context.Context, mjml string) (html string, err error)
}

// VariableRenderer resolves variables in text (subject, preview, body).
type VariableRenderer interface {
	// Render replaces {{ injector.X.Y }} and {{ event.Z }} with actual values.
	Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error)
}
