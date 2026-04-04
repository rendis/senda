#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd npm

SESSION_NAME="senda-template-type-slug-edit-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-template-type-slug-edit.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/ui-template-type-slug-edit.frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/ui-template-type-slug-edit.frontend-dev.log"
REPORT_PATH="$ARTIFACT_DIR/ui-template-type-slug-edit-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-template-type-slug-edit"

TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
WORKSPACE_CODE="${WORKSPACE_CODE:-${SYSTEM_WORKSPACE_CODE:-main}}"
ORIGINAL_SLUG="${TEMPLATE_TYPE_SLUG_EDIT_ORIGINAL:-slug-edit-ui}"
UPDATED_SLUG="${TEMPLATE_TYPE_SLUG_EDIT_UPDATED:-slug-edit-ui-v2}"
CONFLICT_SLUG="${TEMPLATE_TYPE_SLUG_EDIT_CONFLICT:-slug-edit-ui-conflict}"
FIXTURE_NAME="${TEMPLATE_TYPE_SLUG_EDIT_NAME:-Slug Edit Fixture}"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

stop_frontend_dev() {
  if [[ -f "$FRONTEND_PID_FILE" ]]; then
    local pid
    pid="$(cat "$FRONTEND_PID_FILE")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      log "ui-template-type-slug-edit: stopping frontend dev pid=$pid"
      kill "$pid" >/dev/null 2>&1 || true
      wait "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$FRONTEND_PID_FILE"
  fi
}

cleanup() {
  timeout 5s agent-browser --session "$SESSION_NAME" close >/dev/null 2>&1 || true
  stop_frontend_dev
}

trap cleanup EXIT

frontend_env() {
  local api_url="$SENDA_BASE_URL"
  local auth_secret="${AUTH_SECRET:-ysf1mCbeKS9WIY7kan1OOXg/8MmK35YVZRC9qsYUYFM=}"
  local auth_oidc_issuer="${AUTH_OIDC_ISSUER:-$KEYCLOAK_BASE_URL/realms/senda}"
  local auth_oidc_id="${AUTH_OIDC_ID:-senda-web}"
  local auth_oidc_secret="${AUTH_OIDC_SECRET:-senda-dev-secret}"

  NEXT_PUBLIC_API_URL="$api_url" \
    AUTH_URL="$FRONTEND_BASE_URL" \
    AUTH_SECRET="$auth_secret" \
    AUTH_TRUST_HOST=true \
    AUTH_OIDC_ISSUER="$auth_oidc_issuer" \
    AUTH_OIDC_ID="$auth_oidc_id" \
    AUTH_OIDC_SECRET="$auth_oidc_secret" \
    "$@"
}

start_frontend_dev() {
  load_env_report "$ENV_REPORT_FILE"

  if [[ -f "$FRONTEND_PID_FILE" ]] && kill -0 "$(cat "$FRONTEND_PID_FILE")" >/dev/null 2>&1; then
    log "ui-template-type-slug-edit: frontend dev already running (pid=$(cat "$FRONTEND_PID_FILE"))"
    return 0
  fi

  stop_stale_frontend_listeners

  log "ui-template-type-slug-edit: starting frontend dev server"
  (
    cd "$ROOT_DIR"
    frontend_env npm --prefix web run dev -- --hostname 0.0.0.0 --port 3000
  ) >"$FRONTEND_LOG_FILE" 2>&1 &

  local pid=$!
  echo "$pid" >"$FRONTEND_PID_FILE"

  log "ui-template-type-slug-edit: waiting frontend health"
  local ok=0
  for _ in $(seq 1 120); do
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "frontend dev exited before becoming healthy, log: $FRONTEND_LOG_FILE" >&2
      return 1
    fi
    if curl -fsS "$FRONTEND_BASE_URL/login" >/dev/null 2>&1; then
      ok=1
      break
    fi
    sleep 1
  done

  if [[ "$ok" -ne 1 ]]; then
    echo "frontend dev failed to start, log: $FRONTEND_LOG_FILE" >&2
    return 1
  fi

  log "ui-template-type-slug-edit: frontend dev ready"
}

issue_test_token() {
  local email="$1"
  go run "$ROOT_DIR/cmd/systemtest" token \
    --email "$email" \
    --secret "$SENDA_E2E_JWT_SECRET" \
    | tail -n1
}

management_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token="${WORKSPACE_ADMIN_TOKEN:-}"

  if [[ -z "$token" ]]; then
    token="$(issue_test_token "$WORKSPACE_ADMIN_EMAIL")"
    if [[ -z "$token" || "$token" == "null" ]]; then
      echo "failed to obtain workspace-admin test token for management API calls" >&2
      return 1
    fi
    WORKSPACE_ADMIN_TOKEN="$token"
  fi

  if [[ -n "$body" ]]; then
    curl -sS -w '\n%{http_code}' -X "$method" "$SENDA_BASE_URL$path" \
      -H "Authorization: Bearer $token" \
      -H 'Content-Type: application/json' \
      --data "$body"
  else
    curl -sS -w '\n%{http_code}' -X "$method" "$SENDA_BASE_URL$path" \
      -H "Authorization: Bearer $token"
  fi
}

management_api_expect() {
  local expected_status="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local response
  response="$(management_api_request "$method" "$path" "$body")"
  local status
  status="$(printf '%s' "$response" | tail -n1)"
  local payload
  payload="$(printf '%s' "$response" | sed '$d')"
  if [[ "$status" != "$expected_status" ]]; then
    echo "management ${method} failed: expected=${expected_status} actual=${status} path=${path} body=${payload}" >&2
    return 1
  fi
  printf '%s\n' "$payload"
}

management_api_status() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local response
  response="$(management_api_request "$method" "$path" "$body")"
  printf '%s' "$response" | tail -n1
}

ensure_template_type_fixture() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/template-types"

  local base_status
  base_status="$(management_api_status GET "${base_path}/${ORIGINAL_SLUG}")"
  if [[ "$base_status" == "200" ]]; then
    return 0
  fi

  local updated_status
  updated_status="$(management_api_status GET "${base_path}/${UPDATED_SLUG}")"
  if [[ "$updated_status" == "200" ]]; then
    management_api_expect "200" PUT "${base_path}/${UPDATED_SLUG}" "{\"slug\":\"${ORIGINAL_SLUG}\",\"name\":\"${FIXTURE_NAME}\"}" >/dev/null
    return 0
  fi

  management_api_expect "201" POST "${base_path}" "{\"slug\":\"${ORIGINAL_SLUG}\",\"name\":\"${FIXTURE_NAME}\"}" >/dev/null
}

ensure_conflict_template_type() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/template-types"
  local status
  status="$(management_api_status GET "${base_path}/${CONFLICT_SLUG}")"
  if [[ "$status" == "200" ]]; then
    return 0
  fi

  management_api_expect "201" POST "${base_path}" "{\"slug\":\"${CONFLICT_SLUG}\",\"name\":\"${FIXTURE_NAME} Conflict\"}" >/dev/null
}

ensure_workspace_admin_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-template-type-slug-edit: loading saved browser state"
    ab state load "$STATE_FILE" >/dev/null
    return 0
  fi

  log "ui-template-type-slug-edit: logging in as workspace admin"
  ab open "$FRONTEND_BASE_URL/login" >/dev/null
  ab wait 1200 >/dev/null

  if ! ab_json eval '(() => {
    const buttons = Array.from(document.querySelectorAll("button"));
    const button = buttons.find((candidate) => {
      const text = (candidate.textContent || "").replace(/\s+/g, " ").trim();
      return /sign in|oidc|iniciar|ingresar/i.test(text);
    });
    if (!button) return "missing";
    button.click();
    return "clicked";
  })()' | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "login button not found on frontend login page" >&2
    return 1
  fi

  ab wait "#username" >/dev/null
  ab fill "#username" "$WORKSPACE_ADMIN_EMAIL" >/dev/null
  ab fill "#password" "$WORKSPACE_ADMIN_PASSWORD" >/dev/null
  ab eval "(function(){var f=document.querySelector('#kc-form-login'); if (f) { f.submit(); return 'submitted'; } var b=document.querySelector('#kc-login'); if (b) { b.click(); return 'clicked'; } return 'missing'; })()" >/dev/null

  local settled=0
  for _ in $(seq 1 40); do
    local current_url
    current_url="$(ab_json get url | jq -r '.data.url // ""')"
    if [[ "$current_url" == "$FRONTEND_BASE_URL"* ]] && [[ "$current_url" != *"/api/auth/"* ]]; then
      settled=1
      break
    fi
    sleep 1
  done

  if [[ "$settled" -ne 1 ]]; then
    echo "workspace admin login did not return to frontend" >&2
    return 1
  fi

  ab state save "$STATE_FILE" >/dev/null
}

wait_for_text() {
  local needle="$1"
  for _ in $(seq 1 40); do
    local body
    body="$(ab_json eval '(() => (document.body?.innerText || "").replace(/\s+/g, " ").trim())()' | jq -r '.data.result // ""')"
    if [[ "$body" == *"$needle"* ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for body text: $needle" >&2
  return 1
}

wait_for_eval_true() {
  local expression="$1"
  for _ in $(seq 1 40); do
    if ab_json eval "$expression" | jq -e '.data.result == true' >/dev/null; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for eval true: $expression" >&2
  return 1
}

click_button_by_aria_label() {
  local label="$1"
  local label_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const label = ${label_json};
    const button = Array.from(document.querySelectorAll('button')).find((candidate) => candidate.getAttribute('aria-label') === label);
    if (!button) return 'missing';
    button.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "button with aria-label not found: ${label}" >&2
    return 1
  fi
}

load_env_report "$ENV_REPORT_FILE"
start_frontend_dev
ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships
ensure_template_type_fixture
ensure_conflict_template_type
ensure_workspace_admin_login

TARGET_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates"
log "ui-template-type-slug-edit: opening $TARGET_URL"
ab open "$TARGET_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "$ORIGINAL_SLUG"
ab screenshot "$SCREENSHOT_DIR/list-before-edit.png" >/dev/null

click_button_by_aria_label "Edit template type ${ORIGINAL_SLUG}"
ab wait "#edit-tt-slug" >/dev/null
wait_for_text "Change the name, slug, adapter, and sender assigned to this template type."

wait_for_eval_true "(() => {
  const input = document.querySelector('#edit-tt-slug');
  const warning = document.body?.innerText?.includes('Cambiar el slug puede romper referencias existentes.') ?? false;
  const resetButton = document.querySelector('button[aria-label=\"Restaurar slug original\"]');
  const submitButton = Array.from(document.querySelectorAll('button')).find((button) => (button.textContent || '').trim() === 'Update');
  return !!input && input.value === '${ORIGINAL_SLUG}' && !warning && !resetButton && submitButton && !submitButton.disabled;
})()"

ab fill "#edit-tt-slug" "$UPDATED_SLUG" >/dev/null
wait_for_text "Cambiar el slug puede romper referencias existentes."
wait_for_eval_true "(() => {
  const resetButton = document.querySelector('button[aria-label=\"Restaurar slug original\"]');
  return !!resetButton;
})()"
ab screenshot "$SCREENSHOT_DIR/modal-slug-dirty.png" >/dev/null

ab fill "#edit-tt-slug" "$ORIGINAL_SLUG" >/dev/null
wait_for_eval_true "(() => {
  const warning = document.body?.innerText?.includes('Cambiar el slug puede romper referencias existentes.') ?? false;
  const resetButton = document.querySelector('button[aria-label=\"Restaurar slug original\"]');
  return !warning && !resetButton;
})()"

ab fill "#edit-tt-slug" "$UPDATED_SLUG" >/dev/null
wait_for_text "Cambiar el slug puede romper referencias existentes."
ab find role button click --name "Restaurar slug original" >/dev/null
wait_for_eval_true "(() => {
  const input = document.querySelector('#edit-tt-slug');
  const resetButton = document.querySelector('button[aria-label=\"Restaurar slug original\"]');
  const active = document.activeElement;
  return !!input && input.value === '${ORIGINAL_SLUG}' && input === active && !resetButton;
})()"

ab fill "#edit-tt-slug" "AB" >/dev/null
wait_for_text "Mínimo 3 caracteres"
wait_for_eval_true "(() => {
  const warning = document.body?.innerText?.includes('Cambiar el slug puede romper referencias existentes.') ?? false;
  const submitButton = Array.from(document.querySelectorAll('button')).find((button) => (button.textContent || '').trim() === 'Update');
  return warning && !!submitButton && submitButton.disabled;
})()"

ab fill "#edit-tt-slug" "$UPDATED_SLUG" >/dev/null
wait_for_text "Cambiar el slug puede romper referencias existentes."
wait_for_eval_true "(() => {
  const submitButton = Array.from(document.querySelectorAll('button')).find((button) => (button.textContent || '').trim() === 'Update');
  return !!submitButton && !submitButton.disabled;
})()"
ab find role button click --name "Update" >/dev/null
wait_for_text "Template type updated"
wait_for_text "$UPDATED_SLUG"
wait_for_eval_true "(() => !document.querySelector('#edit-tt-slug'))()"
ab screenshot "$SCREENSHOT_DIR/list-after-success.png" >/dev/null

click_button_by_aria_label "Edit template type ${UPDATED_SLUG}"
ab wait "#edit-tt-slug" >/dev/null
wait_for_eval_true "(() => {
  const input = document.querySelector('#edit-tt-slug');
  const warning = document.body?.innerText?.includes('Cambiar el slug puede romper referencias existentes.') ?? false;
  return !!input && input.value === '${UPDATED_SLUG}' && !warning;
})()"

ab fill "#edit-tt-slug" "$CONFLICT_SLUG" >/dev/null
ab find role button click --name "Update" >/dev/null
wait_for_text "Failed to update"
wait_for_eval_true "(() => {
  const input = document.querySelector('#edit-tt-slug');
  return !!input && input.value === '${CONFLICT_SLUG}';
})()"
ab screenshot "$SCREENSHOT_DIR/modal-conflict-error.png" >/dev/null

cat >"$REPORT_PATH" <<EOF
# UI Template Type Slug Edit Report

- Route: \`/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates\`
- Original slug: \`${ORIGINAL_SLUG}\`
- Updated slug: \`${UPDATED_SLUG}\`
- Conflict slug: \`${CONFLICT_SLUG}\`

## Covered

- Initial modal state (warning/reset hidden, submit enabled)
- Dirty slug state (warning visible, reset visible)
- Manual revert to original slug
- Reset button restores original slug and returns focus
- Invalid slug disables submit and preserves warning
- Successful save updates list row and persists reopened modal baseline
- Conflict save preserves modal state and shows error feedback

## Artifacts

- \`$SCREENSHOT_DIR/list-before-edit.png\`
- \`$SCREENSHOT_DIR/modal-slug-dirty.png\`
- \`$SCREENSHOT_DIR/list-after-success.png\`
- \`$SCREENSHOT_DIR/modal-conflict-error.png\`
EOF

log "ui-template-type-slug-edit: report written -> $REPORT_PATH"
