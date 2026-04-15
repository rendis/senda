package sdk

import (
	"context"
	"reflect"
	"testing"

	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

const sdkPkgPath = "github.com/rendis/senda/sdk"

func TestPublicContractsOwnedBySDKPackage(t *testing.T) {
	ctx := NewInjectorContext(nil, "", nil, [16]byte{}, [16]byte{}, EnvironmentProd, "")

	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{name: "InjectorContext", typ: reflect.TypeOf(ctx).Elem()},
		{name: "Environment", typ: reflect.TypeOf(Environment(""))},
		{name: "InjectorFieldSpec", typ: reflect.TypeOf(InjectorFieldSpec{})},
		{name: "ResolveFunc", typ: reflect.TypeOf((*ResolveFunc)(nil)).Elem()},
		{name: "InitFunc", typ: reflect.TypeOf((*InitFunc)(nil)).Elem()},
		{name: "ExternalIntegrationRequest", typ: reflect.TypeOf(ExternalIntegrationRequest{})},
		{name: "ExternalPermissions", typ: reflect.TypeOf(ExternalPermissions{})},
		{name: "ExternalAuthResult", typ: reflect.TypeOf(ExternalAuthResult{})},
		{name: "ExternalWorkspaceResolution", typ: reflect.TypeOf(ExternalWorkspaceResolution{})},
		{name: "ExternalAuthMethod", typ: reflect.TypeOf((*ExternalAuthMethod)(nil)).Elem()},
		{name: "ExternalWorkspaceResolver", typ: reflect.TypeOf((*ExternalWorkspaceResolver)(nil)).Elem()},
	}

	for _, tc := range cases {
		if got := tc.typ.PkgPath(); got != sdkPkgPath {
			t.Fatalf("%s should belong to sdk package, got %q", tc.name, got)
		}
	}
}

func TestPublicContractsDoNotLeakInternalDomainEnvironment(t *testing.T) {
	envType := reflect.TypeOf(Environment(""))

	newInjectorContextType := reflect.TypeOf(NewInjectorContext)
	if got := newInjectorContextType.In(5); got != envType {
		t.Fatalf("NewInjectorContext environment parameter = %v, want %v", got, envType)
	}

	ctxType := reflect.TypeOf((*InjectorContext)(nil))
	method, ok := ctxType.MethodByName("Environment")
	if !ok {
		t.Fatal("InjectorContext.Environment method not found")
	}
	if got := method.Type.Out(0); got != envType {
		t.Fatalf("InjectorContext.Environment return type = %v, want %v", got, envType)
	}

	field, ok := reflect.TypeOf(ExternalIntegrationRequest{}).FieldByName("Environment")
	if !ok {
		t.Fatal("ExternalIntegrationRequest.Environment field not found")
	}
	if got := field.Type; got != envType {
		t.Fatalf("ExternalIntegrationRequest.Environment field type = %v, want %v", got, envType)
	}
}

func TestBuildExtensions_AdaptsInjectorRegistrationToInternalContracts(t *testing.T) {
	engine := New()
	engine.RegisterInjector(InjectorRegistration{
		Code:        "student",
		Name:        "Student",
		Description: "Student data",
		Static:      true,
		Fields: []InjectorFieldSpec{{
			Name:        "name",
			Type:        FieldTypeText,
			Description: "Student name",
		}},
		Resolve: func(_ context.Context, injCtx *InjectorContext) (map[string]any, error) {
			return map[string]any{
				"header": injCtx.Header("X-Test"),
				"env":    string(injCtx.Environment()),
			}, nil
		},
	})

	ext := engine.buildExtensions()
	if ext == nil || len(ext.Injectors) != 1 {
		t.Fatalf("expected one adapted injector, got %#v", ext)
	}

	catalogInjector, ok := ext.Injectors[0].(port.CatalogCodeInjector)
	if !ok {
		t.Fatalf("expected adapted injector to expose catalog metadata, got %T", ext.Injectors[0])
	}
	catalog := catalogInjector.Catalog()
	if catalog.Fields[0].Type != domain.InjectorFieldType(FieldTypeText) {
		t.Fatalf("expected catalog field type %q, got %q", FieldTypeText, catalog.Fields[0].Type)
	}

	resolve, _ := ext.Injectors[0].Resolve()
	values, err := resolve(context.Background(), port.NewInjectorContext(
		map[string]string{"X-Test": "ok"},
		"tenant:workspace:welcome",
		nil,
		[16]byte{},
		[16]byte{},
		toDomainEnvironment(EnvironmentProd),
		"welcome",
	))
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if values["header"] != "ok" {
		t.Fatalf("expected wrapped injector context to expose headers, got %#v", values["header"])
	}
	if values["env"] != string(EnvironmentProd) {
		t.Fatalf("expected wrapped injector context to expose environment, got %#v", values["env"])
	}
}

func TestBuildExtensions_AdaptsExternalContractsToInternalContracts(t *testing.T) {
	engine := New().
		RegisterExternalAuthMethod(publicExternalAuthMethod{}).
		RegisterExternalWorkspaceResolver(publicExternalWorkspaceResolver{})

	ext := engine.buildExtensions()
	if ext == nil {
		t.Fatal("expected extensions to be built")
	}
	if len(ext.ExternalAuthMethods) != 1 || len(ext.ExternalWorkspaceResolvers) != 1 {
		t.Fatalf("expected one auth and one resolver, got %d and %d", len(ext.ExternalAuthMethods), len(ext.ExternalWorkspaceResolvers))
	}

	authResult, err := ext.ExternalAuthMethods[0].Authenticate(context.Background(), &port.ExternalIntegrationRequest{
		ProfileSlug: "partner",
		TenantCode:  "acme",
		Headers:     map[string]string{"X-Test": "ok"},
	})
	if err != nil {
		t.Fatalf("auth returned error: %v", err)
	}
	if !authResult.Permissions.BuilderAccess || authResult.Context["tenant"] != "acme" {
		t.Fatalf("unexpected auth result: %#v", authResult)
	}

	resolution, err := ext.ExternalWorkspaceResolvers[0].ResolveWorkspace(context.Background(), &port.ExternalIntegrationRequest{
		ProfileSlug: "partner",
		TenantCode:  "acme",
	}, authResult)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if resolution.WorkspaceCode != "main" || !resolution.ReadOnly {
		t.Fatalf("unexpected workspace resolution: %#v", resolution)
	}
}

type publicExternalAuthMethod struct{}

func (publicExternalAuthMethod) Name() string        { return "signed-header" }
func (publicExternalAuthMethod) Description() string { return "Signed header auth" }
func (publicExternalAuthMethod) Authenticate(_ context.Context, req *ExternalIntegrationRequest) (*ExternalAuthResult, error) {
	return &ExternalAuthResult{
		Permissions: ExternalPermissions{BuilderAccess: true},
		Context: map[string]any{
			"tenant": req.TenantCode,
		},
	}, nil
}

type publicExternalWorkspaceResolver struct{}

func (publicExternalWorkspaceResolver) Name() string        { return "tenant-workspace" }
func (publicExternalWorkspaceResolver) Description() string { return "Tenant workspace resolver" }
func (publicExternalWorkspaceResolver) ResolveWorkspace(_ context.Context, _ *ExternalIntegrationRequest, _ *ExternalAuthResult) (*ExternalWorkspaceResolution, error) {
	return &ExternalWorkspaceResolution{WorkspaceCode: "main", ReadOnly: true}, nil
}
