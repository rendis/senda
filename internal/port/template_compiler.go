package port

import "context"

// TemplateCompiler compiles MJML templates into HTML.
type TemplateCompiler interface {
	// Compile takes MJML source with resolved variables and returns HTML.
	Compile(ctx context.Context, mjml string) (html string, err error)
}

type templateCompilerContextKey string

const templateCompilerPublicBaseURLKey templateCompilerContextKey = "template-compiler-public-base-url"

// WithTemplateCompilerPublicBaseURL overrides the public base URL used by the
// compiler for request-scoped rewrites, such as preview-only media URLs.
func WithTemplateCompilerPublicBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, templateCompilerPublicBaseURLKey, baseURL)
}

// TemplateCompilerPublicBaseURLFromContext extracts the request-scoped public
// base URL override, if present.
func TemplateCompilerPublicBaseURLFromContext(ctx context.Context) (string, bool) {
	value, ok := ctx.Value(templateCompilerPublicBaseURLKey).(string)
	return value, ok
}

// VariableRenderer resolves variables in text (subject, preview, body).
type VariableRenderer interface {
	// Render replaces {{ injector.X.Y }} and {{ event.Z }} with actual values.
	Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error)
}
