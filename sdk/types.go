package sdk

import "github.com/rendis/senda/internal/port"

// InjectorContext is the read-only context passed to code injectors and
// the init function during send resolution. See port.InjectorContext for
// the full method set.
type InjectorContext = port.InjectorContext

// NewInjectorContext creates a new InjectorContext. Re-exported for tests
// and advanced usage; normally the engine creates this internally.
var NewInjectorContext = port.NewInjectorContext
