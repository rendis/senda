#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-}"

if [[ -z "${MODE}" ]]; then
  echo "usage: $0 <backend-pr|backend-main|frontend|pr|main>" >&2
  exit 1
fi

run_backend_pr() {
  echo "==> Backend gate (PR)"
  make lint
  make vet
  make swagger-check
  make test
}

run_backend_main() {
  run_backend_pr
}

run_frontend() {
  echo "==> Frontend gate"
  corepack pnpm --dir web typecheck
  corepack pnpm --dir web lint --max-warnings=0
}

case "${MODE}" in
  backend-pr)
    run_backend_pr
    ;;
  backend-main)
    run_backend_main
    ;;
  frontend)
    run_frontend
    ;;
  pr)
    run_backend_pr
    run_frontend
    ;;
  main)
    run_backend_main
    run_frontend
    ;;
  *)
    echo "unknown mode: ${MODE}" >&2
    exit 1
    ;;
esac
