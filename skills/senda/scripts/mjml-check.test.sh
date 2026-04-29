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

run_case ok-2 0 '<mjml>
  <mj-body>
    <mj-hero mode="fluid-height" background-color="#0f172a">
      <mj-text>Hello</mj-text>
    </mj-hero>
    <mj-section>
      <mj-column>
        <mj-button href="https://example.com">Go</mj-button>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>'

run_case ok-3 0 '<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-raw><div class="x">snippet</div></mj-raw>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>'

run_case fail-1 1 '<!DOCTYPE html>
<mjml>
  <mj-body>
    <mj-section><mj-column><mj-text>hi</mj-text></mj-column></mj-section>
  </mj-body>
</mjml>' "forbidden HTML document tag <!DOCTYPE"

run_case fail-2 1 '<html>
  <head><title>x</title></head>
  <body><mjml><mj-body><mj-section><mj-column><mj-text>hi</mj-text></mj-column></mj-section></mj-body></mjml></body>
</html>' "forbidden HTML root tag <html>"

run_case fail-4 1 '<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <head><meta charset="utf-8"></head>
        <mj-text>hi</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>' "forbidden HTML <head> tag"

run_case fail-5 1 '<mj-body>
  <mj-section><mj-column><mj-text>hi</mj-text></mj-column></mj-section>
</mj-body>' "document must start with <mjml>"

# ---- summary ----
echo "---"
echo "passed: $pass_count, failed: $fail_count"
[[ $fail_count -eq 0 ]] || exit 1
exit 0
