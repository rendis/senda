package sdk

import (
	"time"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

// InjectorFieldType is the public field type enum for injector catalog fields.
type InjectorFieldType = domain.InjectorFieldType

const (
	FieldTypeText   = domain.FieldTypeText
	FieldTypeNumber = domain.FieldTypeNumber
	FieldTypeBool   = domain.FieldTypeBool
	FieldTypeImg    = domain.FieldTypeImg
	FieldTypeURL    = domain.FieldTypeURL
	FieldTypeHTML   = domain.FieldTypeHTML
)

// ResolveFunc executes injector logic and returns field-name → value pairs.
// See port.CodeResolveFunc for the full signature.
type ResolveFunc = port.CodeResolveFunc

// InjectorFieldSpec describes one field exposed by a catalogable injector.
type InjectorFieldSpec = port.InjectorFieldSpec

// InjectorRegistration is the single public registration contract for
// injectors. Static registrations are catalogable and read-only in the UI.
type InjectorRegistration struct {
	Code         string
	Name         string
	Description  string
	Static       bool
	TTL          time.Duration
	Fields       []InjectorFieldSpec
	Resolve      ResolveFunc
	Dependencies []string
	Critical     bool
	Timeout      time.Duration
}

// InitFunc runs once per send request before code injectors.
// See port.CodeInitFunc for the full signature.
type InitFunc = port.CodeInitFunc

// ExternalAuthMethod validates external integration requests and returns a
// normalized access context.
type ExternalAuthMethod = port.ExternalAuthMethod

// ExternalWorkspaceResolver maps a validated request to a workspace code or a
// read-only fallback.
type ExternalWorkspaceResolver = port.ExternalWorkspaceResolver

// ExternalIntegrationRequest is the request context passed to auth and
// resolver extensions.
type ExternalIntegrationRequest = port.ExternalIntegrationRequest

// ExternalPermissions describes the capability flags returned by an auth
// method.
type ExternalPermissions = port.ExternalPermissions

// ExternalAuthResult is the normalized access context returned by auth.
type ExternalAuthResult = port.ExternalAuthResult

// ExternalWorkspaceResolution is the result of a workspace resolver.
type ExternalWorkspaceResolution = port.ExternalWorkspaceResolution
