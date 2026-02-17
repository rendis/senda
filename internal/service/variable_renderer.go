package service

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// variablePattern matches {{ event.X }}, {{ injector.name.field }}, etc.
// Supports optional whitespace inside braces.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}`)

// VariableRenderer replaces {{ event.X }} and {{ injector.name.field }} patterns
// with actual values from event variables and merged injectors.
type VariableRenderer struct{}

// NewVariableRenderer creates a new VariableRenderer.
func NewVariableRenderer() *VariableRenderer {
	return &VariableRenderer{}
}

// Render replaces variable placeholders in a plain text template string.
// - {{ event.X }} is replaced with eventVars["X"]
// - {{ injector.name.field }} is replaced with injectors["name"]["field"]
// Missing values are replaced with empty string.
// No HTML escaping is applied — use RenderHTML for HTML context.
func (r *VariableRenderer) Render(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
	return r.render(template, injectors, eventVars, false)
}

// RenderHTML replaces variable placeholders in an HTML template string.
// All replacement values are HTML-escaped to prevent XSS injection.
// - {{ event.X }} is replaced with html.EscapeString(eventVars["X"])
// - {{ injector.name.field }} is replaced with html.EscapeString(injectors["name"]["field"])
// Missing values are replaced with empty string.
func (r *VariableRenderer) RenderHTML(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
	return r.render(template, injectors, eventVars, true)
}

// render is the internal implementation shared by Render and RenderHTML.
func (r *VariableRenderer) render(template string, injectors map[string]map[string]any, eventVars map[string]any, escapeHTML bool) (string, error) {
	result := variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract the path from {{ path }}
		submatch := variablePattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		path := submatch[1]

		parts := strings.SplitN(path, ".", 2)
		if len(parts) < 2 {
			return ""
		}

		prefix := parts[0]
		rest := parts[1]

		var resolved string

		switch prefix {
		case "event":
			if eventVars == nil {
				return ""
			}
			val, ok := eventVars[rest]
			if !ok || val == nil {
				return ""
			}
			resolved = fmt.Sprintf("%v", val)

		case "injector":
			if injectors == nil {
				return ""
			}
			injParts := strings.SplitN(rest, ".", 2)
			if len(injParts) < 2 {
				return ""
			}
			injName := injParts[0]
			fieldName := injParts[1]
			fields, ok := injectors[injName]
			if !ok || fields == nil {
				return ""
			}
			val, ok := fields[fieldName]
			if !ok || val == nil {
				return ""
			}
			resolved = fmt.Sprintf("%v", val)

		default:
			return ""
		}

		if escapeHTML {
			resolved = html.EscapeString(resolved)
		}
		return resolved
	})

	return result, nil
}
