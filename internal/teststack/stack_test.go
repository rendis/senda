package teststack

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeResourceNamesUsesCanonicalScope(t *testing.T) {
	t.Parallel()

	first := makeResourceNames(ResolveScope(ScopeInput{
		ProjectRoot: "/tmp/spec-autonomous-e2e-isolation",
		Spec:        "stack-smoke",
		Worktree:    "spec-autonomous-e2e-isolation",
		Mode:        ModePR,
		Run:         "run-a",
	}))
	second := makeResourceNames(ResolveScope(ScopeInput{
		ProjectRoot: "/tmp/spec-autonomous-e2e-isolation",
		Spec:        "stack-smoke",
		Worktree:    "spec-autonomous-e2e-isolation",
		Mode:        ModePR,
		Run:         "run-b",
	}))

	if first.Network == second.Network {
		t.Fatalf("expected unique network names across runs")
	}
	if first.Postgres == second.Postgres {
		t.Fatalf("expected unique postgres names across runs")
	}
	for _, name := range []string{first.Network, first.Postgres, first.Mailpit, first.AWSSim, first.AWSSimBackend, first.App} {
		if len(name) > dockerNameMaxLen {
			t.Fatalf("expected docker-safe resource name, got %q", name)
		}
	}
}

func TestDownRequiresReportPath(t *testing.T) {
	t.Parallel()

	if err := Down(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty report path")
	}
}

func TestDownFailsWhenReportIsMissing(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing-report.json")
	err := Down(context.Background(), missing)
	if err == nil {
		t.Fatal("expected error for missing report path")
	}
	if !strings.Contains(err.Error(), "load report") {
		t.Fatalf("expected load report error, got %v", err)
	}
}
