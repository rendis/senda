#!/usr/bin/env bash
# mjml-check.test.sh — fixture-driven tests for mjml-check.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$SCRIPT_DIR/mjml-check.sh"

pass_count=0
fail_count=0

run_case() {
    local name="$1" expected_exit="$2" fixture="$3" expect_stderr_match="${4:-}"

    local stderr_file
    stderr_file=$(mktemp)

    local actual_exit
    set +e
    printf '%s' "$fixture" | "$SCRIPT" - >/dev/null 2>"$stderr_file"
    actual_exit=$?
    set -e

    local stderr_content
    stderr_content=$(<"$stderr_file")
    rm -f "$stderr_file"

    if [[ "$actual_exit" != "$expected_exit" ]]; then
        echo "FAIL $name: expected exit $expected_exit, got $actual_exit"
        echo "--- stderr ---"
        echo "$stderr_content"
        echo "--------------"
        fail_count=$((fail_count + 1))
        return
    fi

    if [[ -n "$expect_stderr_match" ]] && ! grep -qF "$expect_stderr_match" <<<"$stderr_content"; then
        echo "FAIL $name: stderr did not contain '$expect_stderr_match'"
        echo "--- stderr ---"
        echo "$stderr_content"
        echo "--------------"
        fail_count=$((fail_count + 1))
        return
    fi

    echo "OK $name"
    pass_count=$((pass_count + 1))
}

# ---- PASS cases ----

run_case ok-1 0 '<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>hi</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>'

# ---- summary ----
echo "---"
echo "passed: $pass_count, failed: $fail_count"
[[ $fail_count -eq 0 ]] || exit 1
exit 0
