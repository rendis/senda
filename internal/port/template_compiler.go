package port

import "context"

// TemplateCompiler compiles MJML templates into HTML.
type TemplateCompiler interface {
	Compile(ctx context.Context, mjml string) (html string, err error)
}
