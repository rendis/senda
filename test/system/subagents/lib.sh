#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SYSTEM_MODE="${SYSTEM_MODE:-pr}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/artifacts/system/$(date -u +%Y%m%dT%H%M%SZ)}"
SENDA_BASE_URL="${SENDA_BASE_URL:-http://localhost:8090}"
KEYCLOAK_BASE_URL="${KEYCLOAK_BASE_URL:-http://localhost:9090}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-senda}"
KEYCLOAK_ADMIN_USER="${KEYCLOAK_ADMIN_USER:-admin}"
KEYCLOAK_ADMIN_PASS="${KEYCLOAK_ADMIN_PASS:-admin}"
MAILPIT_BASE_URL="${MAILPIT_BASE_URL:-http://localhost:9025}"
FRONTEND_BASE_URL="${FRONTEND_BASE_URL:-http://localhost:3000}"
SENDA_E2E_JWT_SECRET="${SENDA_E2E_JWT_SECRET:-e2e-test-jwt-secret-at-least-32-characters-long}"
SYSTEM_TENANT_CODE="${SYSTEM_TENANT_CODE:-system-test-corp}"
SYSTEM_TENANT_NAME="${SYSTEM_TENANT_NAME:-System Test Corp}"
SYSTEM_WORKSPACE_CODE="${SYSTEM_WORKSPACE_CODE:-system-main}"
SYSTEM_WORKSPACE_NAME="${SYSTEM_WORKSPACE_NAME:-System Main}"

SUPERADMIN_EMAIL="${SUPERADMIN_EMAIL:-admin@senda.dev}"
TENANT_ADMIN_EMAIL="${TENANT_ADMIN_EMAIL:-tenant-admin@senda.dev}"
WORKSPACE_ADMIN_EMAIL="${WORKSPACE_ADMIN_EMAIL:-workspace-admin@senda.dev}"
WORKSPACE_EDITOR_EMAIL="${WORKSPACE_EDITOR_EMAIL:-workspace-editor@senda.dev}"
WORKSPACE_VIEWER_EMAIL="${WORKSPACE_VIEWER_EMAIL:-workspace-viewer@senda.dev}"
NO_MEMBER_EMAIL="${NO_MEMBER_EMAIL:-no-member@senda.dev}"

SUPERADMIN_PASSWORD="${SUPERADMIN_PASSWORD:-admin}"
TENANT_ADMIN_PASSWORD="${TENANT_ADMIN_PASSWORD:-tenant-admin}"
WORKSPACE_ADMIN_PASSWORD="${WORKSPACE_ADMIN_PASSWORD:-workspace-admin}"
WORKSPACE_EDITOR_PASSWORD="${WORKSPACE_EDITOR_PASSWORD:-workspace-editor}"
WORKSPACE_VIEWER_PASSWORD="${WORKSPACE_VIEWER_PASSWORD:-workspace-viewer}"
NO_MEMBER_PASSWORD="${NO_MEMBER_PASSWORD:-no-member}"

mkdir -p "$ARTIFACT_DIR"

log() {
  printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    return 1
  fi
}

start_frontend() {
  local pid_file="$ARTIFACT_DIR/frontend.pid"
  local log_file="$ARTIFACT_DIR/frontend.log"

  if [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" >/dev/null 2>&1; then
    log "frontend already running (pid=$(cat "$pid_file"))"
    return 0
  fi

  log "building frontend"
  npm --prefix "$ROOT_DIR/web" run build >/dev/null

  log "starting frontend server"
  (
    cd "$ROOT_DIR"
    NEXT_PUBLIC_API_URL="$SENDA_BASE_URL" \
    AUTH_SECRET="${AUTH_SECRET:-ysf1mCbeKS9WIY7kan1OOXg/8MmK35YVZRC9qsYUYFM=}" \
    AUTH_TRUST_HOST=true \
    AUTH_OIDC_ISSUER="${AUTH_OIDC_ISSUER:-$KEYCLOAK_BASE_URL/realms/senda}" \
    AUTH_OIDC_ID="${AUTH_OIDC_ID:-senda-web}" \
    AUTH_OIDC_SECRET="${AUTH_OIDC_SECRET:-senda-dev-secret}" \
    npm --prefix web run start -- --port 3000
  ) >"$log_file" 2>&1 &

  local pid=$!
  echo "$pid" >"$pid_file"

  log "waiting frontend health"
  local ok=0
  for _ in $(seq 1 60); do
    if curl -fsS "$FRONTEND_BASE_URL/login" >/dev/null 2>&1; then
      ok=1
      break
    fi
    sleep 1
  done

  if [[ "$ok" -ne 1 ]]; then
    echo "frontend failed to start, log: $log_file" >&2
    return 1
  fi

  log "frontend ready"
}

stop_frontend() {
  local pid_file="$ARTIFACT_DIR/frontend.pid"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      log "stopping frontend pid=$pid"
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$pid_file"
  fi
}

ensure_runtime_env() {
  local runtime_env="${RUNTIME_ENV_FILE:-$ARTIFACT_DIR/runtime.env}"
  if [[ -f "$runtime_env" ]]; then
    return 0
  fi

  log "resolving runtime context -> $runtime_env"
  go run "$ROOT_DIR/cmd/systemtest" resolve-context \
    --base-url "$SENDA_BASE_URL" \
    --secret "$SENDA_E2E_JWT_SECRET" \
    --tenant-code "$SYSTEM_TENANT_CODE" \
    --tenant-name "$SYSTEM_TENANT_NAME" \
    --workspace-code "$SYSTEM_WORKSPACE_CODE" \
    --workspace-name "$SYSTEM_WORKSPACE_NAME" \
    --out "$runtime_env"
}

load_runtime_env() {
  local runtime_env="${RUNTIME_ENV_FILE:-$ARTIFACT_DIR/runtime.env}"
  if [[ ! -f "$runtime_env" ]]; then
    echo "runtime env file not found: $runtime_env" >&2
    return 1
  fi
  set -a
  # shellcheck disable=SC1090
  source "$runtime_env"
  set +a
}

seed_keycloak_users() {
  log "seeding keycloak users"
  if ! go run "$ROOT_DIR/cmd/systemtest" keycloak-seed \
    --base-url "$KEYCLOAK_BASE_URL" \
    --realm "$KEYCLOAK_REALM" \
    --admin-user "$KEYCLOAK_ADMIN_USER" \
    --admin-pass "$KEYCLOAK_ADMIN_PASS" \
    --users "${SUPERADMIN_EMAIL}:${SUPERADMIN_PASSWORD},${TENANT_ADMIN_EMAIL}:${TENANT_ADMIN_PASSWORD},${WORKSPACE_ADMIN_EMAIL}:${WORKSPACE_ADMIN_PASSWORD},${WORKSPACE_EDITOR_EMAIL}:${WORKSPACE_EDITOR_PASSWORD},${WORKSPACE_VIEWER_EMAIL}:${WORKSPACE_VIEWER_PASSWORD},${NO_MEMBER_EMAIL}:${NO_MEMBER_PASSWORD}"; then
    log "keycloak API seeding failed; continuing with realm-imported users"
  fi
}

seed_rbac_memberships() {
  ensure_runtime_env
  load_runtime_env
  log "seeding RBAC member roles"
  go run "$ROOT_DIR/cmd/systemtest" seed-rbac \
    --base-url "$SENDA_BASE_URL" \
    --secret "$SENDA_E2E_JWT_SECRET" \
    --tenant-code "${TENANT_CODE:-test-corp}" \
    --workspace-code "${WORKSPACE_CODE:-main}" \
    --superadmin-email "$SUPERADMIN_EMAIL" \
    --tenant-admin-email "$TENANT_ADMIN_EMAIL" \
    --workspace-admin-email "$WORKSPACE_ADMIN_EMAIL" \
    --workspace-editor-email "$WORKSPACE_EDITOR_EMAIL" \
    --workspace-viewer-email "$WORKSPACE_VIEWER_EMAIL" \
    --no-member-email "$NO_MEMBER_EMAIL"
}

role_email() {
  local role="$1"
  case "$role" in
    superadmin) echo "$SUPERADMIN_EMAIL" ;;
    tenant-admin) echo "$TENANT_ADMIN_EMAIL" ;;
    workspace-admin) echo "$WORKSPACE_ADMIN_EMAIL" ;;
    workspace-editor) echo "$WORKSPACE_EDITOR_EMAIL" ;;
    workspace-viewer) echo "$WORKSPACE_VIEWER_EMAIL" ;;
    no-member) echo "$NO_MEMBER_EMAIL" ;;
    *) echo "$SUPERADMIN_EMAIL" ;;
  esac
}

role_password() {
  local role="$1"
  case "$role" in
    superadmin) echo "$SUPERADMIN_PASSWORD" ;;
    tenant-admin) echo "$TENANT_ADMIN_PASSWORD" ;;
    workspace-admin) echo "$WORKSPACE_ADMIN_PASSWORD" ;;
    workspace-editor) echo "$WORKSPACE_EDITOR_PASSWORD" ;;
    workspace-viewer) echo "$WORKSPACE_VIEWER_PASSWORD" ;;
    no-member) echo "$NO_MEMBER_PASSWORD" ;;
    *) echo "$SUPERADMIN_PASSWORD" ;;
  esac
}

sanitize_route() {
  local route="$1"
  if [[ "$route" == "/" ]]; then
    echo "root"
    return
  fi
  route="${route#/}"
  route="${route//\//__}"
  route="${route//[/}"
  route="${route//]/}"
  route="${route//:/_}"
  echo "$route"
}

resolve_route() {
  local route="$1"
  route="${route//\[tenantCode\]/${TENANT_CODE:-test-corp}}"
  route="${route//\[workspaceCode\]/${WORKSPACE_CODE:-main}}"
  route="${route//\[slug\]/${TEMPLATE_SLUG:-welcome-v1}}"
  route="${route//\[trackingId\]/${TRACKING_ID:-missing-tracking-id}}"
  echo "$route"
}

viewport_size() {
  local viewport="$1"
  case "$viewport" in
    desktop)
      echo "1440 1024"
      ;;
    mobile)
      echo "390 844"
      ;;
    *)
      echo "1440 1024"
      ;;
  esac
}
