# MJML Pre-Submit Validator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic static validator (`skills/senda/scripts/mjml-check.sh`) that the senda skill mandates running before any version POST/PUT, blocking the HTML-wrapper class of error (e.g. `<!DOCTYPE>`/`<html>`/`<body>` smuggled into MJML, including inside `<mj-raw>`).

**Architecture:** Pure-bash CLI script with embedded heredoc-based test runner; both live inside `skills/senda/scripts/` so the skill stays self-contained for external embedders. Skill docs (`building-a-template.md`, `versions-locales-and-builder.md`, `SKILL.md`) updated to make the script a mandatory gate and to document the anti-pattern explicitly. No HTTP, no auth, no external dependencies beyond POSIX shell utilities.

**Tech Stack:** bash, grep, sed, awk. No Go, no Node, no Python, no network.

**Reference spec:** `docs/superpowers/specs/2026-04-29-mjml-pre-submit-validator-design.md`

---

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `skills/senda/scripts/mjml-check.sh` | Create | Static validator. Reads MJML from file or stdin, applies 7 rules, reports all violations to stderr, exits 0/1/2. |
| `skills/senda/scripts/mjml-check.test.sh` | Create | Fixture-driven test runner with 8 cases (3 PASS, 5 FAIL) embedded as heredocs. |
| `skills/senda/references/building-a-template.md` | Modify | Add "Anti-pattern: HTML wrappers" section. Update Engine note about `mj-raw`. Update Composer's checklist to make `mjml-check.sh` mandatory before `preview-mjml`. |
| `skills/senda/references/versions-locales-and-builder.md` | Modify | Add Gotcha bullet pointing at the validator. |
| `skills/senda/SKILL.md` | Modify | Add maintenance row to the decision table. |

---

## Task 1: Bootstrap script + test runner with first PASS case

**Files:**
- Create: `skills/senda/scripts/mjml-check.sh`
- Create: `skills/senda/scripts/mjml-check.test.sh`

- [ ] **Step 1: Write the failing test runner with case `ok-1`**

Create `skills/senda/scripts/mjml-check.test.sh`:

```bash
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
```

- [ ] **Step 2: Make it executable and run — expect failure (script does not exist)**

Run:
```bash
chmod +x skills/senda/scripts/mjml-check.test.sh
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `FAIL ok-1: expected exit 0, got 127` (or similar — script not found).

- [ ] **Step 3: Create minimal `mjml-check.sh` that always exits 0**

Create `skills/senda/scripts/mjml-check.sh`:

```bash
#!/usr/bin/env bash
# mjml-check.sh — pre-submit static validator for Senda MJML bodies.
# Exit 0 = OK, exit 1 = rule violation, exit 2 = internal error.
#
# Usage:
#   mjml-check.sh <path-to-mjml>
#   mjml-check.sh -                # read from stdin

set -uo pipefail

usage() {
    {
        echo "usage: $(basename "$0") <path-to-mjml>"
        echo "       $(basename "$0") -                # read from stdin"
    } >&2
    exit 2
}

[[ $# -eq 1 ]] || usage

if [[ "$1" == "-" ]]; then
    SRC=$(cat)
else
    [[ -f "$1" ]] || { echo "mjml-check: internal error: file not found: $1" >&2; exit 2; }
    SRC=$(<"$1")
fi

violations=0

# (rules added in subsequent tasks)

if [[ $violations -gt 0 ]]; then
    echo "mjml-check: $violations violation(s)" >&2
    exit 1
fi
exit 0
```

- [ ] **Step 4: Make it executable and rerun tests — expect PASS**

Run:
```bash
chmod +x skills/senda/scripts/mjml-check.sh
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `OK ok-1` and `passed: 1, failed: 0`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add skills/senda/scripts/mjml-check.sh skills/senda/scripts/mjml-check.test.sh
git commit -m "feat(skill-senda): scaffold mjml-check validator and test runner"
```

---

## Task 2: Reject HTML document tags (rules 1–5)

**Files:**
- Modify: `skills/senda/scripts/mjml-check.sh`
- Modify: `skills/senda/scripts/mjml-check.test.sh`

- [ ] **Step 1: Add failing test cases `fail-1`, `fail-2`, `fail-4`, plus `ok-2`, `ok-3`**

Append to `skills/senda/scripts/mjml-check.test.sh`, immediately before the `# ---- summary ----` block:

```bash
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
```

- [ ] **Step 2: Run tests — expect 3 NEW failures (fail-1/2/4 want exit 1, current script returns 0)**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `OK ok-1`, `OK ok-2`, `OK ok-3`, then three failures for `fail-1`, `fail-2`, `fail-4` (each got exit 0, expected 1). Exit 1.

- [ ] **Step 3: Implement rules 1–5 in `mjml-check.sh`**

In `skills/senda/scripts/mjml-check.sh`, replace the line `# (rules added in subsequent tasks)` with:

```bash
# Rules 1–5: forbidden HTML document tags.
# Each rule: pattern, label, hint.
report() {
    local line="$1" label="$2" hint="$3"
    {
        echo "mjml-check: FAIL line $line: $label"
        echo "  $hint"
        echo "  See skills/senda/references/building-a-template.md \"Anti-pattern: HTML wrappers\"."
    } >&2
    violations=$((violations + 1))
}

scan_pattern() {
    local pattern="$1" label="$2" hint="$3"
    local hits
    hits=$(printf '%s\n' "$SRC" | grep -niE "$pattern" || true)
    [[ -z "$hits" ]] && return
    while IFS=: read -r line _; do
        report "$line" "$label" "$hint"
    done <<<"$hits"
}

WRAPPER_HINT='MJML compiles INTO HTML. Wrapping MJML in HTML is double-wrapping.'
HEAD_HINT='Document head tags belong in <mj-head>, not the body.'

scan_pattern '<!DOCTYPE'        'forbidden HTML document tag <!DOCTYPE'                "$WRAPPER_HINT"
scan_pattern '<html[[:space:]/>]'   'forbidden HTML root tag <html>'                   "$WRAPPER_HINT"
scan_pattern '</html>'              'forbidden HTML root tag </html>'                  "$WRAPPER_HINT"
scan_pattern '(^|[^-])<head[[:space:]>]'  'forbidden HTML <head> tag (use <mj-head>)'  "$HEAD_HINT"
scan_pattern '(^|[^-])</head>'      'forbidden HTML </head> tag (use </mj-head>)'      "$HEAD_HINT"
scan_pattern '(^|[^-])<body[[:space:]>]'  'forbidden HTML <body> tag (use <mj-body>)'  "$WRAPPER_HINT"
scan_pattern '(^|[^-])</body>'      'forbidden HTML </body> tag (use </mj-body>)'      "$WRAPPER_HINT"
scan_pattern '<meta[[:space:]/>]'   'forbidden HTML head tag <meta>'                   "$HEAD_HINT"
scan_pattern '<title[[:space:]/>]'  'forbidden HTML head tag <title>'                  'Use <mj-title> inside <mj-head> if you need a document title.'
scan_pattern '<link[[:space:]/>]'   'forbidden HTML head tag <link>'                   'Use <mj-style> instead of <link>.'
scan_pattern '<base[[:space:]/>]'   'forbidden HTML head tag <base>'                   "$HEAD_HINT"
```

- [ ] **Step 4: Run tests — expect all 6 cases pass**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `OK ok-1`, `OK ok-2`, `OK ok-3`, `OK fail-1`, `OK fail-2`, `OK fail-4`. `passed: 6, failed: 0`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add skills/senda/scripts/mjml-check.sh skills/senda/scripts/mjml-check.test.sh
git commit -m "feat(skill-senda): reject HTML document tags in mjml-check"
```

---

## Task 3: Require `<mjml>` document root (rule 6)

**Files:**
- Modify: `skills/senda/scripts/mjml-check.sh`
- Modify: `skills/senda/scripts/mjml-check.test.sh`

- [ ] **Step 1: Add failing case `fail-5`**

Append to `mjml-check.test.sh` immediately before the `# ---- summary ----` block:

```bash
run_case fail-5 1 '<mj-body>
  <mj-section><mj-column><mj-text>hi</mj-text></mj-column></mj-section>
</mj-body>' "document must start with <mjml>"
```

- [ ] **Step 2: Run tests — expect `fail-5` to fail (current script returns 0 because none of rules 1–5 match)**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: 6 OK + `FAIL fail-5: expected exit 1, got 0`. Exit 1.

- [ ] **Step 3: Implement rule 6**

In `skills/senda/scripts/mjml-check.sh`, append after the `scan_pattern` block:

```bash
# Rule 6: document must start with <mjml> (whitespace and HTML comments allowed before)
# and end with </mjml> (whitespace allowed after).
stripped=$(printf '%s' "$SRC" | awk '
    BEGIN { in_comment=0 }
    {
        line=$0
        # strip leading whitespace
        sub(/^[[:space:]]+/, "", line)
        # consume single-line <!-- ... --> comments at the head
        while (match(line, /^<!--[^-]*(-[^-]+)*-->/)) {
            line = substr(line, RLENGTH + 1)
            sub(/^[[:space:]]+/, "", line)
        }
        if (length(line) > 0) { print line; exit }
    }
')

if [[ ! "$stripped" =~ ^\<mjml($|[[:space:]>]) ]]; then
    {
        echo "mjml-check: FAIL root: document must start with <mjml>"
        echo "  See skills/senda/references/building-a-template.md \"Document skeleton\"."
    } >&2
    violations=$((violations + 1))
fi

# Trim trailing whitespace and check for </mjml> as the final non-whitespace token.
trailing=$(printf '%s' "$SRC" | awk '
    { lines[NR] = $0 }
    END {
        for (i = NR; i >= 1; i--) {
            line = lines[i]
            sub(/[[:space:]]+$/, "", line)
            if (length(line) > 0) { print line; exit }
        }
    }
')

if [[ ! "$trailing" =~ \</mjml\>$ ]]; then
    {
        echo "mjml-check: FAIL root: document must end with </mjml>"
        echo "  See skills/senda/references/building-a-template.md \"Document skeleton\"."
    } >&2
    violations=$((violations + 1))
fi
```

- [ ] **Step 4: Run tests — expect all 7 cases pass**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `passed: 7, failed: 0`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add skills/senda/scripts/mjml-check.sh skills/senda/scripts/mjml-check.test.sh
git commit -m "feat(skill-senda): require <mjml> root in mjml-check"
```

---

## Task 4: `<mj-raw>` clarifier on the reported case (rule 7)

**Files:**
- Modify: `skills/senda/scripts/mjml-check.sh`
- Modify: `skills/senda/scripts/mjml-check.test.sh`

- [ ] **Step 1: Add failing case `fail-3` — the exact reported error**

Append to `mjml-check.test.sh` immediately before the `# ---- summary ----` block:

```bash
run_case fail-3 1 '<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-raw><!DOCTYPE html><html><head></head><body>x</body></html></mj-raw>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>' "<mj-raw> is for small HTML snippets, not full documents"
```

- [ ] **Step 2: Run tests — expect `fail-3` to fail the assertion**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: 7 OK + `FAIL fail-3: stderr did not contain '<mj-raw> is for small HTML snippets, not full documents'`. The exit-code part already passes (the existing rules catch `<!DOCTYPE`, etc.) but the clarifier message is missing. Exit 1.

- [ ] **Step 3: Implement rule 7 — append clarifier when violation lands inside `<mj-raw>` range**

In `skills/senda/scripts/mjml-check.sh`, replace the existing `report` function with:

```bash
# Pre-compute line ranges that fall inside <mj-raw>...</mj-raw> blocks.
# Sets MJ_RAW_LINES (newline-separated list of line numbers inside any mj-raw).
MJ_RAW_LINES=$(printf '%s\n' "$SRC" | awk '
    BEGIN { depth = 0 }
    {
        line = $0
        # naive: count opens and closes per line. Multiple per line ok.
        opens = gsub(/<mj-raw[[:space:]>]/, "&", line)
        closes = gsub(/<\/mj-raw>/, "&", line)
        # If line opens a raw block, the line itself is "inside" from the open onward.
        if (depth > 0 || opens > 0) print NR
        depth += opens - closes
        if (depth < 0) depth = 0
    }
')

is_inside_mj_raw() {
    local line="$1"
    grep -qxF "$line" <<<"$MJ_RAW_LINES"
}

report() {
    local line="$1" label="$2" hint="$3"
    {
        echo "mjml-check: FAIL line $line: $label"
        echo "  $hint"
        if is_inside_mj_raw "$line"; then
            echo "  <mj-raw> is for small HTML snippets, not full documents."
        fi
        echo "  See skills/senda/references/building-a-template.md \"Anti-pattern: HTML wrappers\"."
    } >&2
    violations=$((violations + 1))
}
```

- [ ] **Step 4: Run tests — expect all 8 cases pass**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `passed: 8, failed: 0`, exit 0. All 3 PASS cases (`ok-1`, `ok-2`, `ok-3`) and all 5 FAIL cases (`fail-1`, `fail-2`, `fail-3`, `fail-4`, `fail-5`) succeed. The `fail-3` stderr now contains the `<mj-raw>` clarifier line.

- [ ] **Step 5: Manual smoke test — pipe the reported case directly**

Run:
```bash
printf '%s' '<mjml>
  <mj-body>
    <mj-section><mj-column>
      <mj-raw><!DOCTYPE html><html><head></head><body>x</body></html></mj-raw>
    </mj-column></mj-section>
  </mj-body>
</mjml>' | bash skills/senda/scripts/mjml-check.sh -
echo "exit=$?"
```

Expected stderr contains lines mentioning `<!DOCTYPE`, `<html>`, `<head>`, `</head>`, `<body>`, `</body>`, `</html>`, each followed by the wrapper hint and the `<mj-raw>` clarifier. Final line: `mjml-check: 7 violation(s)`. `exit=1`.

- [ ] **Step 6: Commit**

```bash
git add skills/senda/scripts/mjml-check.sh skills/senda/scripts/mjml-check.test.sh
git commit -m "feat(skill-senda): clarify <mj-raw> abuse in mjml-check violations"
```

---

## Task 5: Update `building-a-template.md` — anti-pattern + checklist

**Files:**
- Modify: `skills/senda/references/building-a-template.md`

- [ ] **Step 1: Update the `mj-raw` mention in the "Engine and version" section**

Locate the bullet (around line 14–17):
```markdown
  `mjml`, `mj-body`, `mj-section`, `mj-column`, `mj-text`, `mj-button`,
  `mj-image`, `mj-divider`, `mj-spacer`, `mj-hero`. Other gomjml-supported
  tags (`mj-head`, `mj-attributes`, `mj-style`, `mj-class`, `mj-raw`,
  `mj-table`, `mj-social`, …) compile fine but are not produced by the UI;
  preview them before publishing.
```

Replace with:
```markdown
  `mjml`, `mj-body`, `mj-section`, `mj-column`, `mj-text`, `mj-button`,
  `mj-image`, `mj-divider`, `mj-spacer`, `mj-hero`. Other gomjml-supported
  tags (`mj-head`, `mj-attributes`, `mj-style`, `mj-class`, `mj-raw`,
  `mj-table`, `mj-social`, …) compile fine but are not produced by the UI;
  preview them before publishing. **`<mj-raw>` accepts small HTML snippets
  only, never a full HTML document — see "Anti-pattern: HTML wrappers" below.**
```

- [ ] **Step 2: Insert the anti-pattern section immediately after "Document skeleton"**

Locate the "Document skeleton" section (around lines 74–98). Immediately after the bullet list ending with `Stack multiple sections vertically. Add ...`, insert a new section:

```markdown
## Anti-pattern: HTML wrappers

**MJML compiles INTO HTML.** Wrapping MJML in HTML is double-wrapping and
breaks the gomjml/XML parser at runtime. Do **not** add any of these to a
`body_mjml`, anywhere in the document, including inside `<mj-raw>`:
`<!DOCTYPE>`, `<html>`, `<head>`, `<body>` (literal — `<mj-body>` is fine),
`<meta>`, `<title>`, `<link>`, `<base>`.

```mjml
<!-- WRONG — runtime XML parsing error -->
<mjml>
  <mj-body>
    <mj-raw>
      <!DOCTYPE html>
      <html><head><meta charset="utf-8"></head>
        <body>Hi {{ event.first_name }}</body>
      </html>
    </mj-raw>
  </mj-body>
</mjml>
```

```mjml
<!-- RIGHT — let MJML produce the HTML wrapper for you -->
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hi {{ event.first_name }}</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>
```

`<mj-raw>` is for small inline HTML snippets the builder cannot express
(e.g. a one-off `<div class="x">…</div>`), never a full document. If you
need a document title or stylesheet, use `<mj-head>` with `<mj-title>` /
`<mj-style>`.
```

- [ ] **Step 3: Update the Composer's checklist to mandate `mjml-check.sh`**

Locate the "Composer's checklist" section near the bottom (around line 277). Replace:
```markdown
3. Run `POST .../preview-mjml` with the current `body_mjml`. Confirm:
   - HTML output looks right.
   - Static-injector previews (locked fields with `allow_overwrite = false`)
     are filled in. Override-able fields stay as `{{ ... }}` in preview —
     that is expected.
```

With:
```markdown
3. Run `bash skills/senda/scripts/mjml-check.sh <file>` (or pipe the body
   to `mjml-check.sh -`). It must exit 0 before you submit any version
   POST/PUT or locale upsert. The script catches the HTML-wrapper class of
   error (`<!DOCTYPE>`, `<html>`, `<body>`, etc., including inside
   `<mj-raw>`) that gomjml only surfaces at compile time.
4. Run `POST .../preview-mjml` with the current `body_mjml`. Confirm:
   - HTML output looks right.
   - Static-injector previews (locked fields with `allow_overwrite = false`)
     are filled in. Override-able fields stay as `{{ ... }}` in preview —
     that is expected.
```

Then renumber the subsequent steps: what was "4. Run `POST .../test-send`..." becomes "5.", and "5. `POST .../publish`..." becomes "6.".

- [ ] **Step 4: Verify the file still parses as Markdown and re-read your edits**

Run:
```bash
grep -n "Anti-pattern: HTML wrappers" skills/senda/references/building-a-template.md
grep -n "mjml-check.sh" skills/senda/references/building-a-template.md
```

Expected: at least one hit each, with the anti-pattern section present and the checklist mentioning the script.

- [ ] **Step 5: Commit**

```bash
git add skills/senda/references/building-a-template.md
git commit -m "docs(skill-senda): document HTML-wrapper anti-pattern and mandatory mjml-check"
```

---

## Task 6: Update `versions-locales-and-builder.md` — Gotcha bullet

**Files:**
- Modify: `skills/senda/references/versions-locales-and-builder.md`

- [ ] **Step 1: Locate the Gotchas section**

Open `skills/senda/references/versions-locales-and-builder.md`. The "Gotchas" heading is near the bottom (around line 163).

- [ ] **Step 2: Add a new bullet at the top of Gotchas**

Insert as the first bullet under `## Gotchas`:

```markdown
- **Pre-submit gate**: run `bash skills/senda/scripts/mjml-check.sh <file>`
  on the candidate `body_mjml` before any version `POST`/`PUT` or locale
  upsert. It blocks the HTML-wrapper class of error (`<!DOCTYPE>`,
  `<html>`, `<body>`, etc., including inside `<mj-raw>`) that
  `preview-mjml` only catches at compile time and that the visual editor's
  XML parser may surface as a runtime error. See
  `building-a-template.md` "Anti-pattern: HTML wrappers".
```

- [ ] **Step 3: Verify**

Run:
```bash
grep -n "mjml-check.sh" skills/senda/references/versions-locales-and-builder.md
```

Expected: at least one hit inside the Gotchas section.

- [ ] **Step 4: Commit**

```bash
git add skills/senda/references/versions-locales-and-builder.md
git commit -m "docs(skill-senda): add mjml-check pre-submit gotcha"
```

---

## Task 7: Update `SKILL.md` — maintenance row

**Files:**
- Modify: `skills/senda/SKILL.md`

- [ ] **Step 1: Locate the decision/maintenance table**

Open `skills/senda/SKILL.md`. Find the table that maps "If you change X → update Y". (Per CLAUDE.md, this is the canonical maintenance table for the skill.)

- [ ] **Step 2: Add a new row to the table**

Add this row (preserve the existing table column count and pipe alignment):

```markdown
| MJML composition rules (allowed/forbidden tags, `<mj-raw>` usage) | `skills/senda/scripts/mjml-check.sh` patterns + `skills/senda/scripts/mjml-check.test.sh` fixtures |
```

If the table uses different column headers, adapt the row's content accordingly while preserving the intent: a change to MJML composition rules requires updating the validator and its tests in the same PR.

- [ ] **Step 3: Verify**

Run:
```bash
grep -n "mjml-check" skills/senda/SKILL.md
```

Expected: at least one hit referencing the script and one referencing the test runner.

- [ ] **Step 4: Commit**

```bash
git add skills/senda/SKILL.md
git commit -m "docs(skill-senda): add mjml-check to maintenance table"
```

---

## Task 8: Final acceptance + taxonomy gate

**Files:** none modified — verification only.

- [ ] **Step 1: Re-run the test suite end-to-end**

Run:
```bash
bash skills/senda/scripts/mjml-check.test.sh
```

Expected: `passed: 8, failed: 0`, exit 0.

- [ ] **Step 2: Run the documented anti-pattern through the script as a smoke test**

Run:
```bash
printf '%s' '<mjml>
  <mj-body>
    <mj-section><mj-column>
      <mj-raw>
        <!DOCTYPE html>
        <html><head><meta charset="utf-8"></head>
          <body>Hi</body>
        </html>
      </mj-raw>
    </mj-column></mj-section>
  </mj-body>
</mjml>' | bash skills/senda/scripts/mjml-check.sh -
echo "exit=$?"
```

Expected: stderr lists multiple violations (DOCTYPE, html, head, meta, body, /head, /body, /html), each annotated with the `<mj-raw>` clarifier line and the building-a-template reference. Final summary `mjml-check: N violation(s)`. `exit=1`.

- [ ] **Step 3: Confirm a real, valid template still passes**

Run:
```bash
bash skills/senda/scripts/mjml-check.sh - <<'EOF'
<mjml>
  <mj-body>
    <mj-section background-color="#ffffff">
      <mj-column>
        <mj-image src="{{ injector.brand.logo_url }}" width="160px" alt="logo" />
        <mj-text>Hi <strong>{{ event.first_name }}</strong>.</mj-text>
        <mj-button href="{{ event.confirmation_url }}">Confirm</mj-button>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>
EOF
echo "exit=$?"
```

Expected: no stderr output. `exit=0`.

- [ ] **Step 4: Run the taxonomy gate (per CLAUDE.md, mandatory when the change touches docs/helper scripts)**

Run:
```bash
make ci-taxonomy-check
```

Expected: target completes with no errors. If it complains about new files not being indexed, follow its error message to add the entries; do not silence the check.

- [ ] **Step 5: Confirm git history**

Run:
```bash
git log --oneline -10
```

Expected: at least 7 new commits with `feat(skill-senda):` and `docs(skill-senda):` prefixes covering the script, test runner, and three doc updates. No `Co-Authored-By` lines (per CLAUDE.md).

- [ ] **Step 6: No additional commit needed unless taxonomy step required updates**

If `make ci-taxonomy-check` required new entries, commit them:

```bash
git add <whatever-it-asked-you-to-add>
git commit -m "chore(skill-senda): wire mjml-check into taxonomy index"
```

Otherwise this task is complete with no commit.

---

## Notes for the executor

- **No `make lint`/`vet`/`test`** — this change touches only bash and markdown. The Go gates do not apply. The script's own tests (`mjml-check.test.sh`) are the validation.
- **Branch handling** — at the time of writing the spec, the working branch was `fix/visual-editor-preserves-unsupported-mjml`. Confirm with the user whether to keep these commits there or move them to a `docs/`-prefixed branch before opening the PR.
- **No `shellcheck`** — not currently part of the repo's gates. If you add one later, make it its own change.
- **Whitespace in heredocs** — the test runner uses `printf '%s'` (not `echo`) to preserve leading whitespace exactly as written. Do not "fix" that to `echo`.
- **macOS vs Linux** — the script must work on both. `grep -niE` and `awk` flags used here are POSIX-compatible. If a step fails on macOS due to GNU-only flags, fall back to portable equivalents (`sed -E` instead of `sed -r`, etc.).
