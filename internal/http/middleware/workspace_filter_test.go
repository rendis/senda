package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/rendis/senda/internal/domain"
)

// fakeWorkspaceExistenceStore is a manual fake for port.WorkspaceExistenceStore.
type fakeWorkspaceExistenceStore struct {
	calledWith struct {
		tenantCode     string
		workspaceCodes []string
		environment    domain.Environment
	}
	returnResult map[string]bool
	returnErr    error
}

func (f *fakeWorkspaceExistenceStore) ExistsActiveByTenantCode(
	_ context.Context,
	tenantCode string,
	workspaceCodes []string,
	environment domain.Environment,
) (map[string]bool, error) {
	f.calledWith.tenantCode = tenantCode
	f.calledWith.workspaceCodes = workspaceCodes
	f.calledWith.environment = environment
	return f.returnResult, f.returnErr
}

func TestWorkspaceFilter_Exists_ForwardsTenantAndEnvironment(t *testing.T) {
	store := &fakeWorkspaceExistenceStore{
		returnResult: map[string]bool{"ws-a": true, "ws-b": false},
	}
	filter := newWorkspaceFilter(store, "acme", domain.EnvironmentProd)

	result, err := filter.Exists(context.Background(), []string{"ws-a", "ws-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.calledWith.tenantCode != "acme" {
		t.Errorf("tenantCode = %q, want %q", store.calledWith.tenantCode, "acme")
	}
	if store.calledWith.environment != domain.EnvironmentProd {
		t.Errorf("environment = %q, want %q", store.calledWith.environment, domain.EnvironmentProd)
	}
	if result["ws-a"] != true || result["ws-b"] != false {
		t.Errorf("result = %v, want {ws-a:true, ws-b:false}", result)
	}
}

func TestWorkspaceFilter_Exists_NilCodesReturnsEmptyWithoutCallingStore(t *testing.T) {
	store := &fakeWorkspaceExistenceStore{}
	filter := newWorkspaceFilter(store, "acme", domain.EnvironmentProd)

	result, err := filter.Exists(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
	if store.calledWith.tenantCode != "" {
		t.Error("store should not have been called for nil codes")
	}
}

func TestWorkspaceFilter_Exists_EmptyCodesReturnsEmptyWithoutCallingStore(t *testing.T) {
	store := &fakeWorkspaceExistenceStore{}
	filter := newWorkspaceFilter(store, "acme", domain.EnvironmentProd)

	result, err := filter.Exists(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
	if store.calledWith.tenantCode != "" {
		t.Error("store should not have been called for empty codes")
	}
}

func TestWorkspaceFilter_Exists_StoreErrorPropagates(t *testing.T) {
	storeErr := errors.New("db connection lost")
	store := &fakeWorkspaceExistenceStore{returnErr: storeErr}
	filter := newWorkspaceFilter(store, "acme", domain.EnvironmentProd)

	_, err := filter.Exists(context.Background(), []string{"ws-a"})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected storeErr, got %v", err)
	}
}
