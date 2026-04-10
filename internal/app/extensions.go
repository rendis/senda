package app

import "github.com/rendis/senda/internal/port"

// Extensions holds user-provided extensions registered via the SDK Engine.
// When nil or empty, Senda runs with built-in behavior only.
type Extensions struct {
	Injectors                  []port.CodeInjector
	InitFunc                   port.CodeInitFunc
	ExternalAuthMethods        []port.ExternalAuthMethod
	ExternalWorkspaceResolvers []port.ExternalWorkspaceResolver
}
