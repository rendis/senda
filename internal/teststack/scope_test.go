package teststack

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestResolveScope_DefaultsFromProjectRootAndMode(t *testing.T) {
	t.Parallel()

	scope, err := resolveScope(Options{
		ProjectRoot: "/tmp/.worktrees/spec-autonomous-e2e-isolation",
		Mode:        ModeNightly,
	})
	if err != nil {
		t.Fatalf("resolveScope() error = %v", err)
	}

	want := Scope{
		Spec:     "systemtest",
		Worktree: "spec-autonomous-e2e-isolation",
		Mode:     ModeNightly,
		Run:      "local",
	}
	if scope != want {
		t.Fatalf("resolveScope() = %#v, want %#v", scope, want)
	}
}

func TestResolveScope_ExplicitFieldsOverrideDefaults(t *testing.T) {
	t.Parallel()

	scope, err := resolveScope(Options{
		ProjectRoot: "/tmp/.worktrees/spec-autonomous-e2e-isolation",
		Mode:        ModePR,
		Scope: Scope{
			Spec:     "autonomous-e2e-isolation",
			Worktree: " worker/spec-autonomous-e2e-isolation ",
			Mode:     ModeNightly,
			Run:      " run-20260411 ",
		},
	})
	if err != nil {
		t.Fatalf("resolveScope() error = %v", err)
	}

	want := Scope{
		Spec:     "autonomous-e2e-isolation",
		Worktree: "worker/spec-autonomous-e2e-isolation",
		Mode:     ModeNightly,
		Run:      "run-20260411",
	}
	if scope != want {
		t.Fatalf("resolveScope() = %#v, want %#v", scope, want)
	}
}

func TestResolveScope_PreservesCustomHarnessMode(t *testing.T) {
	t.Parallel()

	scope := ResolveScope(ScopeInput{
		ProjectRoot: "/tmp/.worktrees/spec-autonomous-e2e-isolation",
		Spec:        "core-e2e",
		Worktree:    "spec-autonomous-e2e-isolation",
		Mode:        Mode("e2e"),
		Run:         "pid-1234",
	})

	if scope.Mode != Mode("e2e") {
		t.Fatalf("ResolveScope() mode = %q, want %q", scope.Mode, Mode("e2e"))
	}
	if got := scope.DockerName("net"); !strings.Contains(got, "-e2e-") {
		t.Fatalf("DockerName() = %q, want custom mode token preserved", got)
	}
}

func TestScopeRuntimeReportAndResourceNamesAreCanonical(t *testing.T) {
	t.Parallel()

	scope := Scope{
		Spec:     "autonomous-e2e-isolation",
		Worktree: "spec-autonomous-e2e-isolation",
		Mode:     ModeNightly,
		Run:      "run-20260411",
	}

	runtime := scope.RuntimeReport()
	if runtime.Spec != scope.Spec || runtime.Worktree != scope.Worktree || runtime.Mode != string(scope.Mode) || runtime.Run != scope.Run {
		t.Fatalf("RuntimeReport() = %#v, want scope fields preserved", runtime)
	}
	if runtime.Hash != "2b174008a43d" {
		t.Fatalf("RuntimeReport().Hash = %q, want %q", runtime.Hash, "2b174008a43d")
	}

	names := makeResourceNames(scope)
	want := resourceNames{
		Network:       "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-net",
		Postgres:      "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-postgres",
		Keycloak:      "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-keycloak",
		Mailpit:       "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-mailpit",
		AWSSim:        "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-aws-sim",
		AWSSimBackend: "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-aws-sim-bk",
		App:           "senda-e2e-ntl-autono-spec-aut-run-20-2b174008a43d-app",
	}
	if names != want {
		t.Fatalf("makeResourceNames() = %#v, want %#v", names, want)
	}
}

func TestScopeResourceNamesRemainDockerSafeForMessyInputs(t *testing.T) {
	t.Parallel()

	scope := Scope{
		Spec:     " Autonomous E2E Isolation / nightly ",
		Worktree: "feature/spec_autonomous.e2e.isolation",
		Mode:     ModePR,
		Run:      "Run #2026/04/11",
	}

	names := makeResourceNames(scope)
	matcher := regexp.MustCompile(`^[a-z0-9-]+$`)
	for label, value := range map[string]string{
		"network":         names.Network,
		"postgres":        names.Postgres,
		"keycloak":        names.Keycloak,
		"mailpit":         names.Mailpit,
		"aws_sim":         names.AWSSim,
		"aws_sim_backend": names.AWSSimBackend,
		"app":             names.App,
	} {
		if !matcher.MatchString(value) {
			t.Fatalf("%s name %q is not docker-safe", label, value)
		}
		if len(value) > 63 {
			t.Fatalf("%s name length = %d, want <= 63", label, len(value))
		}
	}
}

func TestDown_UsesScopeFromReportInsteadOfModeFallback(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "docker.log")
	fakeDocker := filepath.Join(tempDir, "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE_DOCKER_LOG\"\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("FAKE_DOCKER_LOG", logPath)

	scope := Scope{
		Spec:     "autonomous-e2e-isolation",
		Worktree: "spec-autonomous-e2e-isolation",
		Mode:     ModeNightly,
		Run:      "run-20260411",
	}
	reportPath := filepath.Join(tempDir, "env-report.json")
	if err := writeReport(reportPath, &Report{
		Mode: string(scope.Mode),
		Runtime: RuntimeReport{
			Scope: scope.RuntimeReport(),
		},
	}); err != nil {
		t.Fatalf("writeReport(): %v", err)
	}

	if err := Down(context.Background(), reportPath); err != nil {
		t.Fatalf("Down() error = %v", err)
	}

	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("Down() should preserve report file, stat err = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake docker log: %v", err)
	}
	logOutput := string(logData)
	names := makeResourceNames(scope)
	for _, want := range []string{
		"rm -f " + names.App,
		"rm -f " + names.AWSSim,
		"rm -f " + names.AWSSimBackend,
		"rm -f " + names.Mailpit,
		"rm -f " + names.Keycloak,
		"rm -f " + names.Postgres,
		"network rm " + names.Network,
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("Down() log missing %q\nfull log:\n%s", want, logOutput)
		}
	}
	if strings.Contains(logOutput, "senda-stack-pr") || strings.Contains(logOutput, "senda-stack-nightly") {
		t.Fatalf("Down() used legacy destructive fallback unexpectedly:\n%s", logOutput)
	}
}
