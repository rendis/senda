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
