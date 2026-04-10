package sdk

import "github.com/rendis/senda/internal/port"

// Injector is the interface users implement to provide custom injectable
// values. Resolved fields merge with DB injectors and become available
// in templates as {{ injector.CODE.field }}.
//
// See port.CodeInjector for the full method contract.
type Injector = port.CodeInjector

// ResolveFunc executes injector logic and returns field-name → value pairs.
// See port.CodeResolveFunc for the full signature.
type ResolveFunc = port.CodeResolveFunc

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
