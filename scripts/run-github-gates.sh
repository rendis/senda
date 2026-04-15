#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"

if [[ -z "${MODE}" ]]; then
  echo "usage: $0 <backend-pr|frontend|pr>" >&2
  exit 1
fi

run_backend_pr() {
  echo "==> Backend gate (PR)"
  make lint
  make vet
  make swagger-check
  make test
}

run_frontend() {
  echo "==> Frontend gate"
  corepack pnpm --dir web test
  corepack pnpm --dir web typecheck
  corepack pnpm --dir web lint --max-warnings=0
}

case "${MODE}" in
  backend-pr)
    run_backend_pr
    ;;
  frontend)
    run_frontend
    ;;
  pr)
    run_backend_pr
    run_frontend
    ;;
  *)
    echo "unknown mode: ${MODE}" >&2
    exit 1
    ;;
esac
