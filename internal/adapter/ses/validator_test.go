package ses

import (
	"strings"
	"testing"
)

func TestValidationProbeName_GeneratesUniqueNames(t *testing.T) {
	const prefix = "senda-validate-perm-check"

	first := validationProbeName(prefix)
	second := validationProbeName(prefix)

	if first == second {
		t.Fatalf("expected unique probe names, got %q twice", first)
	}
	if !strings.HasPrefix(first, prefix+"-") {
		t.Fatalf("expected %q to start with %q", first, prefix+"-")
	}
	if !strings.HasPrefix(second, prefix+"-") {
		t.Fatalf("expected %q to start with %q", second, prefix+"-")
	}
	if first == prefix || second == prefix {
		t.Fatalf("probe names must not reuse the fixed legacy name %q", prefix)
	}
}

func TestValidationProbeTopicARN_UsesProbeName(t *testing.T) {
	got := validationProbeTopicARN("us-east-1", "senda-validate-perm-check-abc123")

	if got != "arn:aws:sns:us-east-1:000000000000:senda-validate-perm-check-abc123" {
		t.Fatalf("unexpected probe topic arn %q", got)
	}
}
