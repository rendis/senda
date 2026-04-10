package domain

import (
	"fmt"
	"strings"
)

// Environment identifies the operational workspace environment.
type Environment string

const (
	EnvironmentProd Environment = "prod"
	EnvironmentTest Environment = "test"
)

// Environments returns the supported environments in stable order.
func Environments() []Environment {
	return []Environment{EnvironmentProd, EnvironmentTest}
}

// ParseEnvironment validates and normalizes a raw environment string.
func ParseEnvironment(raw string) (Environment, error) {
	switch Environment(strings.ToLower(strings.TrimSpace(raw))) {
	case EnvironmentProd:
		return EnvironmentProd, nil
	case EnvironmentTest:
		return EnvironmentTest, nil
	default:
		return "", fmt.Errorf("invalid environment %q", raw)
	}
}

// MustParseEnvironment panics when the value is invalid. Intended for internal constants/tests.
func MustParseEnvironment(raw string) Environment {
	env, err := ParseEnvironment(raw)
	if err != nil {
		panic(err)
	}
	return env
}

// Valid reports whether the environment is one of the supported values.
func (e Environment) Valid() bool {
	return e == EnvironmentProd || e == EnvironmentTest
}

// String returns the canonical string value.
func (e Environment) String() string {
	return string(e)
}

// APIKeyTokenPrefix returns the raw API key prefix expected in Authorization headers.
func (e Environment) APIKeyTokenPrefix() string {
	return "senda_" + e.String() + "_"
}

// APIKeyPrefix returns the persisted prefix metadata stored with API keys.
func (e Environment) APIKeyPrefix() string {
	return "senda_" + e.String()
}
