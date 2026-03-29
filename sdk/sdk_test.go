package sdk_test

import (
	"context"
	"testing"
	"time"

	"github.com/rendis/senda/sdk"
)

// testInjector is a minimal Injector implementation for compile verification.
type testInjector struct{}

func (t *testInjector) Code() string { return "test_inj" }
func (t *testInjector) Resolve() (sdk.ResolveFunc, []string) {
	return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
		return map[string]any{
			"greeting": "Hello from code injector!",
			"source":   injCtx.Header("X-Source"),
		}, nil
	}, nil
}
func (t *testInjector) IsCritical() bool        { return false }
func (t *testInjector) Timeout() time.Duration   { return 5 * time.Second }

// testDependentInjector depends on testInjector.
type testDependentInjector struct{}

func (t *testDependentInjector) Code() string { return "dependent" }
func (t *testDependentInjector) Resolve() (sdk.ResolveFunc, []string) {
	return func(ctx context.Context, injCtx *sdk.InjectorContext) (map[string]any, error) {
		parent, _ := injCtx.GetResolved("test_inj")
		greeting := ""
		if parent != nil {
			if g, ok := parent["greeting"].(string); ok {
				greeting = g
			}
		}
		return map[string]any{
			"derived": greeting + " (extended)",
		}, nil
	}, []string{"test_inj"}
}
func (t *testDependentInjector) IsCritical() bool        { return true }
func (t *testDependentInjector) Timeout() time.Duration   { return 0 }

func TestEngineRegistration(t *testing.T) {
	engine := sdk.New()

	engine.RegisterInjector(&testInjector{})
	engine.RegisterInjector(&testDependentInjector{})

	engine.SetInitFunc(func(ctx context.Context, injCtx *sdk.InjectorContext) (any, error) {
		return map[string]string{"loaded": "true"}, nil
	})

	startCalled := false
	shutdownCalled := false

	engine.OnStart(func(ctx context.Context) error {
		startCalled = true
		return nil
	})
	engine.OnShutdown(func(ctx context.Context) error {
		shutdownCalled = true
		return nil
	})

	e2 := sdk.NewWithConfig("custom.yaml")
	if e2 == nil {
		t.Fatal("NewWithConfig returned nil")
	}

	_ = startCalled
	_ = shutdownCalled
}

func TestEngineFluentAPI(t *testing.T) {
	engine := sdk.New()

	// Every method should return the same *Engine for chaining.
	got := engine.
		RegisterInjector(&testInjector{}).
		SetInitFunc(func(_ context.Context, _ *sdk.InjectorContext) (any, error) { return nil, nil }).
		OnStart(func(_ context.Context) error { return nil }).
		OnShutdown(func(_ context.Context) error { return nil })

	if got != engine {
		t.Error("fluent methods should return the same *Engine")
	}
}

func TestInjectorContext_NilHeaders(t *testing.T) {
	ctx := sdk.NewInjectorContext(nil, "", nil, [16]byte{}, [16]byte{}, "")

	// Should not panic.
	if got := ctx.Header("X-Anything"); got != "" {
		t.Errorf("Header on nil headers = %q, want empty", got)
	}

	hdrs := ctx.Headers()
	if len(hdrs) != 0 {
		t.Errorf("Headers() on nil = %v, want empty map", hdrs)
	}
}

func TestInjectorContext_HeadersCopy(t *testing.T) {
	original := map[string]string{"Key": "value"}
	ctx := sdk.NewInjectorContext(original, "", nil, [16]byte{}, [16]byte{}, "")

	// Modify the returned copy.
	copy := ctx.Headers()
	copy["Key"] = "mutated"
	copy["New"] = "injected"

	// Original should be unchanged.
	if ctx.Header("Key") != "value" {
		t.Error("Headers() should return a copy, not a reference")
	}
	if ctx.Header("New") != "" {
		t.Error("modifying copy should not affect the original")
	}
}

func TestInjectorContext_Concurrency(t *testing.T) {
	ctx := sdk.NewInjectorContext(nil, "ref", nil, [16]byte{1}, [16]byte{2}, "type")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			ctx.SetResolved("writer", map[string]any{"i": i})
			ctx.SetInitData(i)
		}
	}()

	for i := 0; i < 100; i++ {
		ctx.GetResolved("writer")
		_ = ctx.InitData()
		_ = ctx.AllResolved()
	}
	<-done
}

func TestInjectorContext(t *testing.T) {
	ctx := sdk.NewInjectorContext(
		map[string]string{"X-Case-Id": "case-123", "X-Source": "test"},
		"tenant:workspace:welcome",
		map[string]any{"name": "Jane"},
		[16]byte{1}, [16]byte{2},
		"welcome",
	)

	if got := ctx.Header("X-Case-Id"); got != "case-123" {
		t.Errorf("Header(X-Case-Id) = %q, want %q", got, "case-123")
	}

	if got := ctx.Ref(); got != "tenant:workspace:welcome" {
		t.Errorf("Ref() = %q, want %q", got, "tenant:workspace:welcome")
	}

	vars := ctx.Variables()
	if vars["name"] != "Jane" {
		t.Errorf("Variables()[name] = %v, want %q", vars["name"], "Jane")
	}

	if got := ctx.TemplateType(); got != "welcome" {
		t.Errorf("TemplateType() = %q, want %q", got, "welcome")
	}

	// InitData starts nil.
	if ctx.InitData() != nil {
		t.Error("InitData() should be nil initially")
	}

	// Set and retrieve init data.
	ctx.SetInitData("my-data")
	if ctx.InitData() != "my-data" {
		t.Errorf("InitData() = %v, want %q", ctx.InitData(), "my-data")
	}

	// Resolved starts empty.
	if _, ok := ctx.GetResolved("test"); ok {
		t.Error("GetResolved should return false for unset code")
	}

	// Set and retrieve resolved.
	ctx.SetResolved("brand", map[string]any{"name": "Acme"})
	fields, ok := ctx.GetResolved("brand")
	if !ok {
		t.Fatal("GetResolved should return true after SetResolved")
	}
	if fields["name"] != "Acme" {
		t.Errorf("GetResolved(brand)[name] = %v, want %q", fields["name"], "Acme")
	}

	// MergeDBInjectors.
	ctx.MergeDBInjectors(map[string]map[string]any{
		"company": {"logo": "https://logo.png"},
	})
	all := ctx.AllResolved()
	if all["brand"]["name"] != "Acme" {
		t.Error("AllResolved should include previously set values")
	}
	if all["company"]["logo"] != "https://logo.png" {
		t.Error("AllResolved should include merged DB values")
	}
}
