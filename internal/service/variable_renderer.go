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

// VariableRenderer replaces {{ event.X }}, {{ injector.name.field }}, and
// {{ system.X }} patterns with actual values from the supplied maps.
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
	return r.render(template, injectors, eventVars, nil, false)
}

// RenderWithSystem replaces variable placeholders in a plain text template string,
// including {{ system.X }} which is resolved from systemVars.
// - {{ event.X }} is replaced with eventVars["X"]
// - {{ injector.name.field }} is replaced with injectors["name"]["field"]
// - {{ system.X }} is replaced with systemVars["X"]
// Missing values are replaced with empty string.
func (r *VariableRenderer) RenderWithSystem(template string, injectors map[string]map[string]any, eventVars map[string]any, systemVars map[string]string) (string, error) {
	return r.render(template, injectors, eventVars, systemVars, false)
}

// RenderHTML replaces variable placeholders in an HTML template string.
// All replacement values are HTML-escaped to prevent XSS injection.
// - {{ event.X }} is replaced with html.EscapeString(eventVars["X"])
// - {{ injector.name.field }} is replaced with html.EscapeString(injectors["name"]["field"])
// Missing values are replaced with empty string.
func (r *VariableRenderer) RenderHTML(template string, injectors map[string]map[string]any, eventVars map[string]any) (string, error) {
	return r.render(template, injectors, eventVars, nil, true)
}

// render is the internal implementation shared by Render, RenderWithSystem, and RenderHTML.
func (r *VariableRenderer) render(template string, injectors map[string]map[string]any, eventVars map[string]any, systemVars map[string]string, escapeHTML bool) (string, error) {
	matches := variablePattern.FindAllStringSubmatchIndex(template, -1)
	if len(matches) == 0 {
		return template, nil
	}

	var result strings.Builder
	result.Grow(len(template))
	cursor := 0
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		matchStart, matchEnd := match[0], match[1]
		pathStart, pathEnd := match[2], match[3]
		result.WriteString(template[cursor:matchStart])
		cursor = matchEnd

		path := template[pathStart:pathEnd]
		parts := strings.SplitN(path, ".", 2)
		if len(parts) < 2 {
			continue
		}

		prefix := parts[0]
		rest := parts[1]

		var (
			resolved string
			ok       bool
		)

		switch prefix {
		case "event":
			resolved, ok = resolveEventVar(eventVars, rest)
		case "injector":
			resolved, ok = resolveInjectorVar(injectors, rest)
		case "system":
			// system vars bypass text-node escaping but still need XML attribute
			// escaping when used inside MJML attributes.
			resolved, ok = resolveSystemVar(systemVars, rest), true
		default:
			continue
		}

		if !ok {
			continue
		}
		if escapeHTML || (!escapeHTML && isPlaceholderInXMLAttribute(template, matchStart)) {
			resolved = html.EscapeString(resolved)
		}
		result.WriteString(resolved)
	}
	result.WriteString(template[cursor:])

	return result.String(), nil
}

func isPlaceholderInXMLAttribute(template string, matchStart int) bool {
	if matchStart < 0 || matchStart > len(template) {
		return false
	}

	var quote byte
	tagOpen := false
	for i := 0; i < matchStart; i++ {
		switch template[i] {
		case '"', '\'':
			if !tagOpen {
				continue
			}
			switch quote {
			case 0:
				quote = template[i]
			case template[i]:
				quote = 0
			}
		case '<':
			if quote == 0 {
				tagOpen = true
			}
		case '>':
			if quote == 0 {
				tagOpen = false
			}
		}
	}
	return tagOpen && quote != 0
}

// resolveEventVar returns the string representation of eventVars[key] and whether it was found.
func resolveEventVar(eventVars map[string]any, key string) (string, bool) {
	if eventVars == nil {
		return "", false
	}
	val, ok := eventVars[key]
	if !ok || val == nil {
		return "", false
	}
	return fmt.Sprintf("%v", val), true
}

// resolveInjectorVar resolves {{ injector.name.field }} from the injectors map.
// rest is the portion after the "injector." prefix, i.e. "name.field".
func resolveInjectorVar(injectors map[string]map[string]any, rest string) (string, bool) {
	if injectors == nil {
		return "", false
	}
	injParts := strings.SplitN(rest, ".", 2)
	if len(injParts) < 2 {
		return "", false
	}
	fields, ok := injectors[injParts[0]]
	if !ok || fields == nil {
		return "", false
	}
	val, ok := fields[injParts[1]]
	if !ok || val == nil {
		return "", false
	}
	return fmt.Sprintf("%v", val), true
}

// resolveSystemVar returns systemVars[key], or empty string when systemVars is
// nil or the key is missing.
//
// SECURITY: system variables bypass HTML escaping by design (so signed URLs
// containing "&" query separators survive intact). Callers MUST only pass
// server-generated values such as signed unsubscribe URLs. Never pass
// admin- or user-supplied strings (e.g. workspace_name) without sanitising
// them first; otherwise this becomes an XSS vector when the body is rendered
// as HTML.
func resolveSystemVar(systemVars map[string]string, key string) string {
	if systemVars == nil {
		return ""
	}
	return systemVars[key]
}
