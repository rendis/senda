#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/artifacts/system/batch-flows-$(date -u +%Y%m%dT%H%M%SZ)}"
BACKEND_PID_FILE="$ARTIFACT_DIR/backend.pid"
BACKEND_LOG_FILE="$ARTIFACT_DIR/backend.log"
APP_CONTAINER="${BATCH_E2E_APP_CONTAINER:-docker-senda-1}"
APP_WAS_RUNNING=0

mkdir -p "$ARTIFACT_DIR"

source "$ROOT_DIR/test/system/subagents/lib.sh"

require_cmd go
require_cmd curl
require_cmd jq
require_cmd docker

export ARTIFACT_DIR
export SENDA_BASE_URL="${BATCH_E2E_SENDA_BASE_URL:-http://localhost:8082}"
export MAILPIT_BASE_URL="${BATCH_E2E_MAILPIT_BASE_URL:-http://localhost:8026}"
export KEYCLOAK_BASE_URL="${BATCH_E2E_KEYCLOAK_BASE_URL:-http://localhost:9090}"
export FRONTEND_BASE_URL="${BATCH_E2E_FRONTEND_BASE_URL:-http://localhost:3000}"
export SENDA_DATABASE_URL="${BATCH_E2E_DATABASE_URL:-postgres://senda:senda@localhost:5435/senda?sslmode=disable}"
export SYSTEM_TENANT_CODE="${BATCH_E2E_SYSTEM_TENANT_CODE:-test-corp}"
export SYSTEM_TENANT_NAME="${BATCH_E2E_SYSTEM_TENANT_NAME:-Test Corporation}"
export SYSTEM_WORKSPACE_CODE="${BATCH_E2E_SYSTEM_WORKSPACE_CODE:-main}"
export SYSTEM_WORKSPACE_NAME="${BATCH_E2E_SYSTEM_WORKSPACE_NAME:-Main Workspace}"
export SUPERADMIN_EMAIL="${BATCH_E2E_SUPERADMIN_EMAIL:-superadmin@test.example.com}"
export TENANT_ADMIN_EMAIL="${BATCH_E2E_TENANT_ADMIN_EMAIL:-tenant-admin@test.example.com}"
export WORKSPACE_ADMIN_EMAIL="${BATCH_E2E_WORKSPACE_ADMIN_EMAIL:-ws-admin@test.example.com}"
export WORKSPACE_EDITOR_EMAIL="${BATCH_E2E_WORKSPACE_EDITOR_EMAIL:-ws-editor@test.example.com}"
export WORKSPACE_VIEWER_EMAIL="${BATCH_E2E_WORKSPACE_VIEWER_EMAIL:-ws-viewer@test.example.com}"
export SUPERADMIN_PASSWORD="${BATCH_E2E_SUPERADMIN_PASSWORD:-superadmin-test}"
export TENANT_ADMIN_PASSWORD="${BATCH_E2E_TENANT_ADMIN_PASSWORD:-tenant-admin}"
export WORKSPACE_ADMIN_PASSWORD="${BATCH_E2E_WORKSPACE_ADMIN_PASSWORD:-workspace-admin}"
export WORKSPACE_EDITOR_PASSWORD="${BATCH_E2E_WORKSPACE_EDITOR_PASSWORD:-workspace-editor}"
export WORKSPACE_VIEWER_PASSWORD="${BATCH_E2E_WORKSPACE_VIEWER_PASSWORD:-workspace-viewer}"
export SENDA_E2E_LOGIN_MODE="password_grant"
export SENDA_E2E_KEYCLOAK_BASE_URL="$KEYCLOAK_BASE_URL"
export SENDA_E2E_OIDC_TOKEN_URL="${BATCH_E2E_OIDC_TOKEN_URL:-$KEYCLOAK_BASE_URL/realms/senda/protocol/openid-connect/token}"
export SENDA_E2E_OIDC_TOKEN_HOST="${BATCH_E2E_OIDC_TOKEN_HOST:-}"
export SENDA_E2E_OIDC_CLIENT_ID="${BATCH_E2E_OIDC_CLIENT_ID:-senda-web}"
export SENDA_E2E_OIDC_CLIENT_SECRET="${BATCH_E2E_OIDC_CLIENT_SECRET:-senda-dev-secret}"
export SENDA_E2E_SUPERADMIN_PASSWORD="$SUPERADMIN_PASSWORD"
export SENDA_E2E_TENANT_ADMIN_PASSWORD="$TENANT_ADMIN_PASSWORD"
export SENDA_E2E_WORKSPACE_ADMIN_PASSWORD="$WORKSPACE_ADMIN_PASSWORD"
export SENDA_E2E_WORKSPACE_EDITOR_PASSWORD="$WORKSPACE_EDITOR_PASSWORD"
export SENDA_E2E_WORKSPACE_VIEWER_PASSWORD="$WORKSPACE_VIEWER_PASSWORD"

ensure_password_grant_client() {
  docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh config credentials \
    --server http://localhost:8080 \
    --realm master \
    --user admin \
    --password admin >/dev/null

  local client_id
  client_id="$(
    docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh get clients -r senda -q clientId="$SENDA_E2E_OIDC_CLIENT_ID" \
      | jq -r '.[0].id // empty'
  )"
  if [[ -z "$client_id" ]]; then
    echo "client not found in realm senda: $SENDA_E2E_OIDC_CLIENT_ID" >&2
    return 1
  fi

  docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh update "clients/$client_id" -r senda \
    -s directAccessGrantsEnabled=true >/dev/null
}

ensure_keycloak_user() {
  local email="$1"
  local password="$2"
  local first_name="${3:-E2E}"
  local last_name="${4:-User}"

  local user_id
  user_id="$(
    docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh get users -r senda -q username="$email" \
      | jq -r '.[0].id // empty'
  )"

  if [[ -z "$user_id" ]]; then
    docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh create users -r senda \
      -s username="$email" \
      -s email="$email" \
      -s firstName="$first_name" \
      -s lastName="$last_name" \
      -s enabled=true \
      -s emailVerified=true >/dev/null
    user_id="$(
      docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh get users -r senda -q username="$email" \
        | jq -r '.[0].id // empty'
    )"
  fi

  if [[ -z "$user_id" ]]; then
    echo "failed to resolve Keycloak user id for $email" >&2
    return 1
  fi

  docker exec docker-keycloak-1 /opt/keycloak/bin/kcadm.sh set-password -r senda --userid "$user_id" --new-password "$password" >/dev/null
}

issue_superadmin_token() {
  local curl_args=(
    -fsS
    -X POST "$KEYCLOAK_BASE_URL/realms/senda/protocol/openid-connect/token"
    -H 'Content-Type: application/x-www-form-urlencoded'
    --data-urlencode "client_id=$SENDA_E2E_OIDC_CLIENT_ID"
    --data-urlencode "client_secret=$SENDA_E2E_OIDC_CLIENT_SECRET"
    --data-urlencode 'grant_type=password'
    --data-urlencode 'scope=openid email profile'
    --data-urlencode "username=$SUPERADMIN_EMAIL"
    --data-urlencode "password=$SUPERADMIN_PASSWORD"
  )
  if [[ -n "$SENDA_E2E_OIDC_TOKEN_HOST" ]]; then
    curl_args+=(-H "Host: $SENDA_E2E_OIDC_TOKEN_HOST")
  fi
  curl "${curl_args[@]}" | jq -r '.id_token // .access_token'
}

stop_stale_backend_listeners() {
  local port="${SENDA_BASE_URL##*:}"
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi

  while read -r pid; do
    [[ -n "$pid" ]] || continue
    if kill -0 "$pid" >/dev/null 2>&1; then
      log "batch-flows-e2e: stopping stale backend listener pid=$pid on port ${port}"
      kill "$pid" >/dev/null 2>&1 || true
      sleep 1
      if kill -0 "$pid" >/dev/null 2>&1; then
        kill -9 "$pid" >/dev/null 2>&1 || true
      fi
    fi
  done < <(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
}

start_local_backend() {
  if [[ -f "$BACKEND_PID_FILE" ]] && kill -0 "$(cat "$BACKEND_PID_FILE")" >/dev/null 2>&1; then
    log "batch-flows-e2e: backend already running (pid=$(cat "$BACKEND_PID_FILE"))"
    return 0
  fi

  stop_stale_backend_listeners

  log "batch-flows-e2e: starting local backend from source"
  (
    cd "$ROOT_DIR"
    SENDA_CONFIG="config/config.yaml" \
      SENDA_HOST="127.0.0.1" \
      SENDA_PORT="8082" \
      SENDA_DATABASE_URL="$SENDA_DATABASE_URL" \
      SENDA_MIGRATIONS_PATH="$ROOT_DIR/migrations" \
      SENDA_MASTER_KEY="dev-master-key-change-in-production" \
      SENDA_OIDC_MODE="oidc" \
      SENDA_OIDC_DISCOVERY_URL="$KEYCLOAK_BASE_URL/realms/senda/.well-known/openid-configuration" \
      SENDA_OIDC_CLIENT_ID="senda-web" \
      SENDA_SMTP_HOST="localhost" \
      SENDA_SMTP_PORT="1026" \
      SENDA_TRACKING_BASE_URL="$SENDA_BASE_URL" \
      go run ./cmd/senda
  ) >"$BACKEND_LOG_FILE" 2>&1 &

  local pid=$!
  echo "$pid" >"$BACKEND_PID_FILE"

  for _ in $(seq 1 120); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "local backend exited before becoming healthy, log: $BACKEND_LOG_FILE" >&2
      return 1
    fi
    if curl -fsS "$SENDA_BASE_URL/health" >/dev/null 2>&1; then
      log "batch-flows-e2e: local backend ready"
      return 0
    fi
    sleep 1
  done

  echo "local backend failed to start, log: $BACKEND_LOG_FILE" >&2
  return 1
}

stop_local_backend() {
  if [[ -f "$BACKEND_PID_FILE" ]]; then
    local pid
    pid="$(cat "$BACKEND_PID_FILE")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      log "batch-flows-e2e: stopping local backend pid=$pid"
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$BACKEND_PID_FILE"
  fi
}

reset_test_database() {
  log "batch-flows-e2e: resetting postgres schema in senda database"
  docker exec -i docker-postgres-1 psql -U senda -d senda <<'SQL' >/dev/null
DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public AUTHORIZATION senda;
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_cron";
SQL
}

stop_competing_app() {
  if docker inspect -f '{{.State.Running}}' "$APP_CONTAINER" >/dev/null 2>&1; then
    if [[ "$(docker inspect -f '{{.State.Running}}' "$APP_CONTAINER")" == "true" ]]; then
      APP_WAS_RUNNING=1
      log "batch-flows-e2e: stopping competing app container ${APP_CONTAINER}"
      docker stop "$APP_CONTAINER" >/dev/null
    fi
  fi
}

start_competing_app() {
  if [[ "$APP_WAS_RUNNING" -eq 1 ]]; then
    log "batch-flows-e2e: restarting competing app container ${APP_CONTAINER}"
    docker start "$APP_CONTAINER" >/dev/null
  fi
}

cleanup() {
  stop_local_backend
  start_competing_app
}

trap cleanup EXIT

log "batch-flows-e2e: verifying shared dependencies"
curl -fsS "$MAILPIT_BASE_URL/api/v1/messages" >/dev/null
curl -fsS "$KEYCLOAK_BASE_URL/realms/senda/.well-known/openid-configuration" >/dev/null

stop_competing_app
log "batch-flows-e2e: ensuring Keycloak password-grant client and superadmin user"
ensure_password_grant_client
ensure_keycloak_user "$SUPERADMIN_EMAIL" "$SUPERADMIN_PASSWORD" "Superadmin" "Bootstrap"
ensure_keycloak_user "$WORKSPACE_EDITOR_EMAIL" "$WORKSPACE_EDITOR_PASSWORD" "Workspace" "Editor"
reset_test_database
start_local_backend
SENDA_E2E_SUPERADMIN_TOKEN="$(issue_superadmin_token)"
export SENDA_E2E_SUPERADMIN_TOKEN
if [[ -z "$SENDA_E2E_SUPERADMIN_TOKEN" || "$SENDA_E2E_SUPERADMIN_TOKEN" == "null" ]]; then
  echo "failed to issue static superadmin token for E2E" >&2
  exit 1
fi

log "batch-flows-e2e: running API batch E2E against local stack"
SENDA_E2E_EXTERNAL_STACK=1 \
  SENDA_BASE_URL="$SENDA_BASE_URL" \
  MAILPIT_URL="$MAILPIT_BASE_URL" \
  SENDA_DATABASE_URL="$SENDA_DATABASE_URL" \
  go test -tags=e2e -count=1 -run '^TestF05A_APIBatchSendPerItemContext$' ./test/e2e

log "batch-flows-e2e: running UI bulk-send browser E2E"
"$ROOT_DIR/test/system/subagents/ui-template-bulk-send-tester.sh"

log "batch-flows-e2e: completed successfully (artifact_dir=$ARTIFACT_DIR)"
