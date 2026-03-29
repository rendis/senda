// Package sdk exposes the public API for extending Senda as a library.
//
// Users import this package, create an Engine, register custom injectors
// and lifecycle hooks, then call Run to start the server.
//
// Example:
//
//	engine := sdk.NewWithConfig("config.yaml")
//	engine.RegisterInjector(&MyInjector{})
//	engine.SetInitFunc(myInit())
//	engine.OnStart(func(ctx context.Context) error { return connectDB(ctx) })
//	engine.OnShutdown(func(ctx context.Context) error { return closeDB(ctx) })
//	engine.Run()
package sdk
