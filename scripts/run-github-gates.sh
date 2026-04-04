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
  go vet ./...
  make swagger-check
  make test
  make test-integration
}

run_backend_main() {
  run_backend_pr
  echo "==> Backend gate (main push extras)"
  make test-e2e
}

run_frontend() {
  echo "==> Frontend gate"
  npm --prefix web run typecheck
  npm --prefix web run lint -- --max-warnings=0
  npm --prefix web run build
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
