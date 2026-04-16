package sdk

import (
	"context"
	"testing"

	"github.com/rendis/senda/internal/port"
)

// fakePortWorkspaceFilter is a manual fake for port.WorkspaceFilter that records calls.
type fakePortWorkspaceFilter struct {
	calledWithCodes []string
	returnResult    map[string]bool
	returnErr       error
}

func (f *fakePortWorkspaceFilter) Exists(_ context.Context, codes []string) (map[string]bool, error) {
	f.calledWithCodes = codes
	return f.returnResult, f.returnErr
}

// recordingWorkspaceResolver is an SDK resolver that records the WorkspaceFilter it receives.
type recordingWorkspaceResolver struct {
	receivedFilter WorkspaceFilter
}

func (r *recordingWorkspaceResolver) Name() string        { return "recording" }
func (r *recordingWorkspaceResolver) Description() string { return "records filter" }
func (r *recordingWorkspaceResolver) ResolveWorkspace(_ context.Context, _ *ExternalIntegrationRequest, _ *ExternalAuthResult, filter WorkspaceFilter) (*ExternalWorkspaceResolution, error) {
	r.receivedFilter = filter
	return &ExternalWorkspaceResolution{WorkspaceCode: "main"}, nil
}

// TestWorkspaceFilterBridge_AdapterPassesFilterToSDKResolver verifies that the
// externalWorkspaceResolverAdapter wraps the port.WorkspaceFilter and passes it
// to the SDK resolver as an sdk.WorkspaceFilter.
func TestWorkspaceFilterBridge_AdapterPassesFilterToSDKResolver(t *testing.T) {
	recorder := &recordingWorkspaceResolver{}
	adapter := externalWorkspaceResolverAdapter{resolver: recorder}

	portFilter := &fakePortWorkspaceFilter{
		returnResult: map[string]bool{"ws-a": true},
	}

	_, err := adapter.ResolveWorkspace(context.Background(), &port.ExternalIntegrationRequest{
		TenantCode: "acme",
	}, &port.ExternalAuthResult{}, portFilter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recorder.receivedFilter == nil {
		t.Fatal("expected adapter to pass a WorkspaceFilter to the SDK resolver")
	}

	// Verify the wrapped filter delegates Exists to the port filter.
	result, err := recorder.receivedFilter.Exists(context.Background(), []string{"ws-a"})
	if err != nil {
		t.Fatalf("unexpected error from wrapped filter: %v", err)
	}
	if !result["ws-a"] {
		t.Errorf("expected ws-a=true, got %v", result)
	}
	if len(portFilter.calledWithCodes) != 1 || portFilter.calledWithCodes[0] != "ws-a" {
		t.Errorf("port filter not called with expected codes, got %v", portFilter.calledWithCodes)
	}
}
