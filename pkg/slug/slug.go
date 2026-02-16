package slug

import (
	"fmt"
	"regexp"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,62}[a-z0-9]$`)

var reservedWords = map[string]struct{}{
	"system":    {},
	"admin":     {},
	"api":       {},
	"internal":  {},
	"global":    {},
	"null":      {},
	"undefined": {},
}

// Validate checks if the given string is a valid slug.
// A valid slug is 3-64 characters, starts with a lowercase letter,
// ends with a lowercase letter or digit, and contains only lowercase
// letters, digits, hyphens, and underscores.
func Validate(s string) error {
	if !slugPattern.MatchString(s) {
		return fmt.Errorf("invalid slug %q: must be 3-64 chars, start with letter, end with letter/digit, contain only [a-z0-9_-]", s)
	}
	if _, ok := reservedWords[s]; ok {
		return fmt.Errorf("invalid slug %q: reserved word", s)
	}
	return nil
}
