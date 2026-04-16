package sdk

import (
	"context"
	"testing"
)

type testExternalAuthMethod struct {
	name        string
	description string
}

func (t *testExternalAuthMethod) Name() string        { return t.name }
func (t *testExternalAuthMethod) Description() string { return t.description }
func (t *testExternalAuthMethod) Authenticate(context.Context, *ExternalIntegrationRequest) (*ExternalAuthResult, error) {
	return &ExternalAuthResult{}, nil
}

type testExternalWorkspaceResolver struct {
	name        string
	description string
}

func (t *testExternalWorkspaceResolver) Name() string        { return t.name }
func (t *testExternalWorkspaceResolver) Description() string { return t.description }
func (t *testExternalWorkspaceResolver) ResolveWorkspace(context.Context, *ExternalIntegrationRequest, *ExternalAuthResult, WorkspaceFilter) (*ExternalWorkspaceResolution, error) {
	return &ExternalWorkspaceResolution{}, nil
}

func TestEngineExternalExtensionRegistration(t *testing.T) {
	engine := New()

	auth := &testExternalAuthMethod{name: "signed-header", description: "Validates signed request headers"}
	resolver := &testExternalWorkspaceResolver{name: "workspace-slug", description: "Maps tenant + request to workspace"}

	got := engine.
		RegisterExternalAuthMethod(auth).
		RegisterExternalWorkspaceResolver(resolver)

	if got != engine {
		t.Fatal("fluent external registration should return the same engine")
	}

	ext := engine.buildExtensions()
	if ext == nil {
		t.Fatal("expected buildExtensions to include external registrations")
	}
	if len(ext.ExternalAuthMethods) != 1 {
		t.Fatalf("expected 1 external auth method, got %d", len(ext.ExternalAuthMethods))
	}
	if len(ext.ExternalWorkspaceResolvers) != 1 {
		t.Fatalf("expected 1 external workspace resolver, got %d", len(ext.ExternalWorkspaceResolvers))
	}
	if ext.ExternalAuthMethods[0].Name() != "signed-header" {
		t.Fatalf("unexpected auth method name: %q", ext.ExternalAuthMethods[0].Name())
	}
	if ext.ExternalWorkspaceResolvers[0].Description() != "Maps tenant + request to workspace" {
		t.Fatalf("unexpected resolver description: %q", ext.ExternalWorkspaceResolvers[0].Description())
	}
}

func TestEngineExternalExtensionRegistration_ChainWithExistingAPI(t *testing.T) {
	engine := New()

	got := engine.
		RegisterExternalAuthMethod(&testExternalAuthMethod{name: "auth", description: "desc"}).
		RegisterExternalWorkspaceResolver(&testExternalWorkspaceResolver{name: "resolver", description: "desc"}).
		SetInitFunc(func(_ context.Context, _ *InjectorContext) (any, error) { return nil, nil }).
		OnStart(func(context.Context) error { return nil }).
		OnShutdown(func(context.Context) error { return nil })

	if got != engine {
		t.Fatal("expected fluent chaining to preserve the engine pointer")
	}
}

func TestEngineBuildExtensions_NoExternalRegistrations(t *testing.T) {
	engine := New()

	if ext := engine.buildExtensions(); ext != nil {
		t.Fatalf("expected nil extensions when nothing is registered, got %#v", ext)
	}
}

func TestEngineExternalRegistration_DoesNotAffectExistingInjectorAPIs(t *testing.T) {
	engine := New()
	engine.RegisterInjector(dummyRegistration()).RegisterExternalAuthMethod(&testExternalAuthMethod{name: "auth", description: "desc"})

	ext := engine.buildExtensions()
	if ext == nil {
		t.Fatal("expected extensions to be built")
	}
	if len(ext.Injectors) != 1 {
		t.Fatalf("expected 1 injector, got %d", len(ext.Injectors))
	}
	if len(ext.ExternalAuthMethods) != 1 {
		t.Fatalf("expected 1 external auth method, got %d", len(ext.ExternalAuthMethods))
	}
}

func TestEngineRegisterInjector_StaticRequiresFields(t *testing.T) {
	engine := New()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for static injector without fields")
		}
	}()

	engine.RegisterInjector(InjectorRegistration{
		Code:   "school",
		Static: true,
		Resolve: func(context.Context, *InjectorContext) (map[string]any, error) {
			return map[string]any{"name": "Acme School"}, nil
		},
	})
}

func TestEngineRegisterInjector_DynamicRegistrationUsesCodeNamespace(t *testing.T) {
	engine := New()
	engine.RegisterInjector(InjectorRegistration{
		Code: "runtime_student",
		Name: "Runtime Student",
		Resolve: func(context.Context, *InjectorContext) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	})

	ext := engine.buildExtensions()
	if ext == nil || len(ext.Injectors) != 1 {
		t.Fatal("expected one registered injector")
	}
	if got := ext.Injectors[0].Code(); got != "runtime_student" {
		t.Fatalf("expected code namespace runtime_student, got %q", got)
	}
}

func dummyRegistration() InjectorRegistration {
	return InjectorRegistration{
		Code: "dummy",
		Resolve: func(context.Context, *InjectorContext) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	}
}
