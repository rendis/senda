#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
COMPOSE=(docker compose -f "$ROOT_DIR/docker/docker-compose.yml")
COMPOSE_PID=""
FRONTEND_PID=""
CLEANED_UP=0
API_PORT="${SENDA_PORT:-8081}"
API_URL="${NEXT_PUBLIC_API_URL:-http://localhost:${API_PORT}}"
KEYCLOAK_PORT="${KEYCLOAK_PORT:-9090}"
KEYCLOAK_URL="${AUTH_OIDC_ISSUER:-http://localhost:${KEYCLOAK_PORT}/realms/senda}"
KEYCLOAK_BASE_URL="http://localhost:${KEYCLOAK_PORT}"
MAILPIT_UI_PORT="${MAILPIT_UI_PORT:-8026}"
MAILPIT_SMTP_PORT="${MAILPIT_SMTP_PORT:-1026}"
POSTGRES_PORT="${SENDA_PG_PORT:-5435}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"
FRONTEND_URL="${AUTH_URL:-http://localhost:${FRONTEND_PORT}}"
OIDC_CLIENT_ID="${AUTH_OIDC_ID:-senda-web}"
OIDC_CLIENT_SECRET="${AUTH_OIDC_SECRET:-senda-dev-secret}"

log() {
  printf '[dev] %s\n' "$*"
}

print_runtime_summary() {
  cat <<EOF
[dev] local environment ready
[dev] services
[dev]   API:            ${API_URL}
[dev]   Frontend:       ${FRONTEND_URL}
[dev]   Keycloak:       ${KEYCLOAK_BASE_URL}
[dev]   Keycloak realm: ${KEYCLOAK_URL}
[dev]   Mailpit UI:     http://localhost:${MAILPIT_UI_PORT}
[dev]   Mailpit SMTP:   localhost:${MAILPIT_SMTP_PORT}
[dev]   PostgreSQL:     localhost:${POSTGRES_PORT}
[dev] credentials
[dev]   PostgreSQL:     user=senda password=senda db=senda
[dev]   Keycloak admin: admin / admin
[dev]   OIDC client:    ${OIDC_CLIENT_ID} / ${OIDC_CLIENT_SECRET}
[dev]   Test users:
[dev]     - admin@senda.dev / admin
[dev]     - tenant-admin@senda.dev / tenant-admin
[dev]     - workspace-admin@senda.dev / workspace-admin
[dev]     - workspace-editor@senda.dev / workspace-editor
[dev]     - workspace-viewer@senda.dev / workspace-viewer
[dev]     - no-member@senda.dev / no-member
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

ensure_port_available() {
  local port="$1"
  local label="$2"

  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi

  if lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    echo "$label port $port is already in use; stop the existing process or use a different port before running make dev" >&2
    exit 1
  fi
}

terminate_process_tree() {
  local pid="$1"

  [[ -n "$pid" ]] || return 0
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    return 0
  fi

  if command -v pgrep >/dev/null 2>&1; then
    local child
    while read -r child; do
      [[ -n "$child" ]] || continue
      terminate_process_tree "$child"
    done < <(pgrep -P "$pid" 2>/dev/null || true)
  fi

  kill "$pid" >/dev/null 2>&1 || true
  sleep 1
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill -9 "$pid" >/dev/null 2>&1 || true
  fi
}

wait_for_url() {
  local url="$1"
  local label="$2"
  local attempts="${3:-120}"

  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi

    if [[ -n "$COMPOSE_PID" ]] && ! kill -0 "$COMPOSE_PID" >/dev/null 2>&1; then
      echo "docker compose exited before $label became ready" >&2
      return 1
    fi

    if [[ -n "$FRONTEND_PID" ]] && ! kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
      echo "frontend dev server exited before $label became ready" >&2
      return 1
    fi

    sleep 1
  done

  echo "timed out waiting for $label at $url" >&2
  return 1
}

wait_for_first_exit() {
  while true; do
    if [[ -n "$COMPOSE_PID" ]] && ! kill -0 "$COMPOSE_PID" >/dev/null 2>&1; then
      wait "$COMPOSE_PID"
      return $?
    fi

    if [[ -n "$FRONTEND_PID" ]] && ! kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
      wait "$FRONTEND_PID"
      return $?
    fi

    sleep 1
  done
}

ensure_frontend_deps() {
  if [[ -x "$ROOT_DIR/web/node_modules/.bin/next" ]]; then
    return 0
  fi

  log "frontend dependencies not found; installing with pnpm"
  corepack pnpm --dir "$ROOT_DIR/web" install --frozen-lockfile
}

cleanup() {
  local exit_code="${1:-$?}"

  if [[ "$CLEANED_UP" -eq 1 ]]; then
    return "$exit_code"
  fi
  CLEANED_UP=1

  trap - EXIT INT TERM

  if [[ -n "$FRONTEND_PID" ]]; then
    terminate_process_tree "$FRONTEND_PID"
    wait "$FRONTEND_PID" >/dev/null 2>&1 || true
  fi

  if [[ -n "$COMPOSE_PID" ]] && kill -0 "$COMPOSE_PID" >/dev/null 2>&1; then
    terminate_process_tree "$COMPOSE_PID"
    wait "$COMPOSE_PID" >/dev/null 2>&1 || true
  fi

  log "stopping docker services"
  "${COMPOSE[@]}" down >/dev/null 2>&1 || true

  return "$exit_code"
}

on_signal() {
  cleanup 130
  exit 130
}

trap 'on_signal' INT TERM
trap 'cleanup $?' EXIT

require_cmd docker
require_cmd corepack
require_cmd curl

ensure_port_available "$FRONTEND_PORT" "frontend"
ensure_frontend_deps

log "starting docker services (api, postgres, keycloak, mailpit)"
"${COMPOSE[@]}" up --build &
COMPOSE_PID=$!

log "starting frontend dev server"
(
  cd "$ROOT_DIR"
  NEXT_PUBLIC_API_URL="${API_URL}" \
    AUTH_URL="${FRONTEND_URL}" \
    AUTH_SECRET="${AUTH_SECRET:-dev-auth-secret-at-least-32-characters-long}" \
    AUTH_TRUST_HOST="${AUTH_TRUST_HOST:-true}" \
    AUTH_OIDC_ISSUER="${KEYCLOAK_URL}" \
    AUTH_OIDC_ID="${OIDC_CLIENT_ID}" \
    AUTH_OIDC_SECRET="${OIDC_CLIENT_SECRET}" \
    exec corepack pnpm --dir "$ROOT_DIR/web" exec next dev --hostname 0.0.0.0 --port "${FRONTEND_PORT}"
) &
FRONTEND_PID=$!

wait_for_url "${API_URL}/health" "backend health"
wait_for_url "${KEYCLOAK_BASE_URL}/realms/senda/.well-known/openid-configuration" "Keycloak realm"
wait_for_url "${FRONTEND_URL}/login" "frontend login"

print_runtime_summary

set +e
wait_for_first_exit
exit_code=$?
set -e

cleanup "$exit_code"
exit "$exit_code"
