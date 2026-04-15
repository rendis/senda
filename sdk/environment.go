package sdk

import (
	"fmt"
	"strings"

	"github.com/rendis/senda/internal/domain"
)

// Environment identifies the operational workspace environment in the public SDK.
type Environment string

const (
	EnvironmentProd Environment = "prod"
	EnvironmentTest Environment = "test"
)

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

// Valid reports whether the environment is one of the supported values.
func (e Environment) Valid() bool {
	return e == EnvironmentProd || e == EnvironmentTest
}

// String returns the canonical string value.
func (e Environment) String() string {
	return string(e)
}

func toDomainEnvironment(environment Environment) domain.Environment {
	return domain.Environment(environment)
}

func fromDomainEnvironment(environment domain.Environment) Environment {
	return Environment(environment)
}
