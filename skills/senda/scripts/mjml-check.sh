#!/usr/bin/env bash
# mjml-check.sh — pre-submit static validator for Senda MJML bodies.
# Exit 0 = OK, exit 1 = rule violation, exit 2 = usage/input error.
#
# Usage:
#   mjml-check.sh <path-to-mjml>
#   mjml-check.sh -                # read from stdin

set -euo pipefail

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
    [[ -f "$1" ]] || { echo "mjml-check: file not found: $1" >&2; exit 2; }
    SRC=$(<"$1")
fi

violations=0

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
    local line="$1" status
    set +e
    grep -qxF "$line" <<<"$MJ_RAW_LINES"
    status=$?
    set -e
    if [[ $status -eq 0 ]]; then return 0; fi
    if [[ $status -eq 1 ]]; then return 1; fi
    echo "mjml-check: internal error: grep failed (status=$status) inside is_inside_mj_raw" >&2
    exit 2
}

# Rules 1-5: forbidden HTML document tags.
# Each rule: pattern, label, hint.
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

scan_pattern() {
    local pattern="$1" label="$2" hint="$3"
    local hits status
    set +e
    hits=$(printf '%s\n' "$SRC" | grep -niE "$pattern")
    status=$?
    set -e
    if [[ $status -eq 1 ]]; then
        return  # no matches
    elif [[ $status -ne 0 ]]; then
        echo "mjml-check: internal error: grep failed (status=$status) for pattern: $pattern" >&2
        exit 2
    fi
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

if [[ $violations -gt 0 ]]; then
    echo "mjml-check: $violations violation(s)" >&2
    exit 1
fi
exit 0
