package service

import (
	"fmt"
	"regexp"
	"strings"
)

var previewInjectorPattern = regexp.MustCompile(`\{\{\s*(injector\.[^}]+?)\s*\}\}`)

// RenderStaticInjectorPreview replaces only injector placeholders that exist in
// the provided map, preserving all other tokens verbatim.
func RenderStaticInjectorPreview(template string, injectors map[string]map[string]any) string {
	if template == "" || len(injectors) == 0 {
		return template
	}

	return previewInjectorPattern.ReplaceAllStringFunc(template, func(match string) string {
		submatch := previewInjectorPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		path := strings.TrimSpace(strings.TrimPrefix(submatch[1], "injector."))
		parts := strings.SplitN(path, ".", 2)
		if len(parts) != 2 {
			return match
		}

		fields, ok := injectors[parts[0]]
		if !ok {
			return match
		}

		value, ok := fields[parts[1]]
		if !ok || value == nil {
			return match
		}

		return fmt.Sprintf("%v", value)
	})
}
