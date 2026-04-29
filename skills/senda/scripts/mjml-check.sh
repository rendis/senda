#!/usr/bin/env bash
# mjml-check.sh — pre-submit static validator for Senda MJML bodies.
# Exit 0 = OK, exit 1 = rule violation, exit 2 = usage/input error.
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
    [[ -f "$1" ]] || { echo "mjml-check: file not found: $1" >&2; exit 2; }
    SRC=$(<"$1")
fi

violations=0

# (rules added in subsequent tasks)

if [[ $violations -gt 0 ]]; then
    echo "mjml-check: $violations violation(s)" >&2
    exit 1
fi
exit 0
