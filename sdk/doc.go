// Package sdk exposes the public API for extending Senda as a library.
//
// Users import this package, create an Engine, register public seams, and then
// call Run to start the server.
//
// Example:
//
//	engine := sdk.NewWithConfig("config.yaml")
//	engine.RegisterInjector(sdk.InjectorRegistration{
//		Code: "student",
//		Resolve: func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
//			return map[string]any{"name": "Ada"}, nil
//		},
//	})
//	engine.SetInitFunc(func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error) { return nil, nil })
//	engine.OnStart(func(ctx context.Context) error { return connectDB(ctx) })
//	engine.OnShutdown(func(ctx context.Context) error { return closeDB(ctx) })
//	engine.Run()
package sdk
