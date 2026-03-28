package e2e

import "strings"

func useExternalStackEnv(getenv func(string) string) bool {
	if !truthyEnv(getenv("SENDA_E2E_EXTERNAL_STACK")) {
		return false
	}

	required := []string{
		"SENDA_BASE_URL",
		"MAILPIT_URL",
		"SENDA_DATABASE_URL",
	}
	for _, key := range required {
		if strings.TrimSpace(getenv(key)) == "" {
			return false
		}
	}
	return true
}

func truthyEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
