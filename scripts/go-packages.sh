#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

go list -f '{{.Dir}}' ./... \
  | awk '!/\/web\/node_modules\//' \
  | sed "s|^$ROOT_DIR|.|"
