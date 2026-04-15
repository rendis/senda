package sdk

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/app"
	"github.com/rendis/senda/internal/metrics"
	"github.com/rendis/senda/internal/port"
)

// Engine is the main entry point for running Senda as a library.
// Create one with New or NewWithConfig, register extensions, then call Run.
type Engine struct {
	configPath                 string
	injectors                  []InjectorRegistration
	initFunc                   InitFunc
	externalAuthMethods        []ExternalAuthMethod
	externalWorkspaceResolvers []ExternalWorkspaceResolver
	onStart                    []func(ctx context.Context) error
	onShutdown                 []func(ctx context.Context) error
}

// New creates an Engine with default config path ("config.yaml").
func New() *Engine {
	return &Engine{configPath: "config.yaml"}
}

// NewWithConfig creates an Engine that loads config from the given path.
func NewWithConfig(configPath string) *Engine {
	return &Engine{configPath: configPath}
}

// RegisterInjector adds a custom injector registration. Static registrations
// are catalogable and read-only in the UI; dynamic registrations remain
// runtime-only.
func (e *Engine) RegisterInjector(reg InjectorRegistration) *Engine {
	validateInjectorRegistration(reg)
	e.injectors = append(e.injectors, reg)
	return e
}

// SetInitFunc sets the per-request initialization function.
// Replaces any previously set InitFunc.
func (e *Engine) SetInitFunc(fn InitFunc) *Engine {
	e.initFunc = fn
	return e
}

// RegisterExternalAuthMethod adds a custom external integration auth method.
func (e *Engine) RegisterExternalAuthMethod(method ExternalAuthMethod) *Engine {
	e.externalAuthMethods = append(e.externalAuthMethods, method)
	return e
}

// RegisterExternalWorkspaceResolver adds a custom external integration
// workspace resolver.
func (e *Engine) RegisterExternalWorkspaceResolver(resolver ExternalWorkspaceResolver) *Engine {
	e.externalWorkspaceResolvers = append(e.externalWorkspaceResolvers, resolver)
	return e
}

// OnStart registers a hook that runs after bootstrap, before the server starts.
// Hooks execute synchronously in registration order.
func (e *Engine) OnStart(fn func(ctx context.Context) error) *Engine {
	e.onStart = append(e.onStart, fn)
	return e
}

// OnShutdown registers a hook that runs after the server stops.
// Hooks execute synchronously in reverse order (LIFO).
func (e *Engine) OnShutdown(fn func(ctx context.Context) error) *Engine {
	e.onShutdown = append(e.onShutdown, fn)
	return e
}

// Run loads configuration, bootstraps Senda, starts the server and River workers,
// and blocks until SIGINT/SIGTERM.
func (e *Engine) Run() error {
	cfgPath := e.configPath
	if envPath := os.Getenv("SENDA_CONFIG"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("sdk: load config: %w", err)
	}

	logger := setupLogger(cfg)
	metrics.Register()

	ext := e.buildExtensions()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.Bootstrap(ctx, cfg, logger, ext)
	if err != nil {
		return fmt.Errorf("sdk: bootstrap: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// User OnShutdown hooks in reverse order.
		for i := len(e.onShutdown) - 1; i >= 0; i-- {
			if hookErr := e.onShutdown[i](shutdownCtx); hookErr != nil {
				logger.Error("OnShutdown hook error", "error", hookErr)
			}
		}

		application.Close(shutdownCtx)
	}()

	// User OnStart hooks.
	for _, fn := range e.onStart {
		if hookErr := fn(ctx); hookErr != nil {
			return fmt.Errorf("sdk: OnStart hook: %w", hookErr)
		}
	}

	go func() {
		if riverErr := application.RiverClient.Start(ctx); riverErr != nil {
			logger.Error("river client error", "error", riverErr)
			stop()
		}
	}()

	if err := application.Server.Start(ctx); err != nil {
		logger.Error("server shutdown", "error", err)
	}

	return nil
}

func (e *Engine) buildExtensions() *app.Extensions {
	if len(e.injectors) == 0 && e.initFunc == nil && len(e.externalAuthMethods) == 0 && len(e.externalWorkspaceResolvers) == 0 {
		return nil
	}
	injectors := make([]port.CodeInjector, 0, len(e.injectors))
	for _, reg := range e.injectors {
		injectors = append(injectors, registeredInjector{registration: reg})
	}
	return &app.Extensions{
		Injectors:                  injectors,
		InitFunc:                   adaptInitFunc(e.initFunc),
		ExternalAuthMethods:        adaptExternalAuthMethods(e.externalAuthMethods),
		ExternalWorkspaceResolvers: adaptExternalWorkspaceResolvers(e.externalWorkspaceResolvers),
	}
}

type registeredInjector struct {
	registration InjectorRegistration
}

func (r registeredInjector) displayName() string {
	if r.registration.Name != "" {
		return r.registration.Name
	}
	return r.registration.Code
}
func (r registeredInjector) Code() string { return r.registration.Code }

func (r registeredInjector) Resolve() (port.CodeResolveFunc, []string) {
	return func(ctx context.Context, injCtx *port.InjectorContext) (map[string]any, error) {
		return r.registration.Resolve(ctx, wrapInjectorContext(injCtx))
	}, r.registration.Dependencies
}

func (r registeredInjector) IsCritical() bool { return r.registration.Critical }

func (r registeredInjector) Timeout() time.Duration { return r.registration.Timeout }

func (r registeredInjector) Catalog() port.InjectorCatalog {
	return port.InjectorCatalog{
		Code:        r.registration.Code,
		Name:        r.displayName(),
		Description: r.registration.Description,
		Static:      r.registration.Static,
		TTL:         r.registration.TTL,
		Fields:      adaptInjectorFieldSpecs(r.registration.Fields),
	}
}

func validateInjectorRegistration(reg InjectorRegistration) {
	if reg.Code == "" {
		panic("sdk: injector registration requires Code")
	}
	if reg.Resolve == nil {
		panic("sdk: injector registration requires Resolve")
	}
	if reg.Static && len(reg.Fields) == 0 {
		panic("sdk: static injector registration requires at least one field")
	}
}

func setupLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	_ = level.UnmarshalText([]byte(cfg.Log.Level))

	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler)
}
