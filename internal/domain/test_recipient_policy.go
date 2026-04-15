package domain

import (
	"net/mail"
	"strings"
)

type TestRecipientMode string

const (
	TestRecipientModeReplace TestRecipientMode = "replace"
	TestRecipientModeAppend  TestRecipientMode = "append"
)

func (m TestRecipientMode) Valid() bool {
	return m == TestRecipientModeReplace || m == TestRecipientModeAppend
}

func CanonicalRecipientAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func NormalizeRecipientAddresses(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := CanonicalRecipientAddress(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func ValidateRecipientAddresses(values []string) error {
	for _, value := range NormalizeRecipientAddresses(values) {
		if _, err := mail.ParseAddress(value); err != nil {
			return err
		}
	}
	return nil
}
