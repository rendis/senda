#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd corepack

BODY_TEXT_JS='(() => {
  if (!document.body) return "";
  return (document.body.innerText || "").replace(/\s+/g, " ").trim();
})()'

SESSION_NAME="senda-template-bulk-send-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-bulk-send.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/frontend-dev.log"
UI_REPORT="$ARTIFACT_DIR/ui-template-bulk-send-report.md"
UI_SCREENSHOT_DIR="$ARTIFACT_DIR/ui-template-bulk-send"
INVALID_JSON_FILE="$UI_SCREENSHOT_DIR/invalid-bulk-send.json"
VALID_JSON_FILE="$UI_SCREENSHOT_DIR/valid-bulk-send.json"
TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
WORKSPACE_CODE="${WORKSPACE_CODE:-${SYSTEM_WORKSPACE_CODE:-main}}"
TEMPLATE_TYPE_SLUG="${TEMPLATE_TYPE_SLUG:-welcome-email}"

mkdir -p "$UI_SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

stop_frontend_dev() {
  stop_managed_frontend "$FRONTEND_PID_FILE" "ui-template-bulk-send"
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
  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "ui-template-bulk-send"
}

mailpit_clear() {
  curl -fsS -X DELETE "$MAILPIT_BASE_URL/api/v1/messages" >/dev/null
}

mailpit_wait_for_messages() {
  local expected="$1"
  local timeout_seconds="${2:-30}"
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    local count
    count="$(curl -fsS "$MAILPIT_BASE_URL/api/v1/messages" | jq -r '.messages | length')"
    if [[ "$count" -ge "$expected" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $expected Mailpit messages" >&2
  return 1
}

mailpit_assert_message_contains() {
  local recipient="$1"
  local needle="$2"
  local message_id
  message_id="$(
    curl -fsS "$MAILPIT_BASE_URL/api/v1/messages" \
      | jq -r --arg recipient "$recipient" '.messages[] | select(any(.To[]?; .Address == $recipient)) | .ID' \
      | head -n1
  )"

  if [[ -z "$message_id" ]]; then
    echo "no Mailpit message found for recipient=$recipient" >&2
    return 1
  fi

  local html
  html="$(curl -fsS "$MAILPIT_BASE_URL/api/v1/message/$message_id" | jq -r '.HTML // ""')"
  if [[ "$html" != *"$needle"* ]]; then
    echo "Mailpit message for recipient=$recipient missing text=$needle" >&2
    return 1
  fi
}

issue_oidc_token() {
  local email="$1"
  local password="$2"
  curl -fsS -X POST "$KEYCLOAK_BASE_URL/realms/senda/protocol/openid-connect/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "client_id=${SENDA_E2E_OIDC_CLIENT_ID:-senda-web}" \
    --data-urlencode "client_secret=${SENDA_E2E_OIDC_CLIENT_SECRET:-senda-dev-secret}" \
    --data-urlencode 'grant_type=password' \
    --data-urlencode 'scope=openid email profile' \
    --data-urlencode "username=${email}" \
    --data-urlencode "password=${password}" \
    | jq -r '.id_token // .access_token // empty'
}

management_api_get() {
  local path="$1"
  local token="${WORKSPACE_EDITOR_TOKEN:-}"
  if [[ -z "$token" ]]; then
    token="$(issue_oidc_token "$WORKSPACE_EDITOR_EMAIL" "$WORKSPACE_EDITOR_PASSWORD")"
    if [[ -z "$token" || "$token" == "null" ]]; then
      echo "failed to obtain workspace-editor token for management API calls" >&2
      return 1
    fi
    WORKSPACE_EDITOR_TOKEN="$token"
  fi
  local response
  response="$(
    curl -sS -w '\n%{http_code}' "$SENDA_BASE_URL$path" \
      -H "Authorization: Bearer $token"
  )"
  local status
  status="$(printf '%s' "$response" | tail -n1)"
  local body
  body="$(printf '%s' "$response" | sed '$d')"
  if [[ "$status" != "200" ]]; then
    echo "management GET failed: status=${status} path=${path} body=${body}" >&2
    return 1
  fi
  printf '%s\n' "$body"
}

resolve_editor_query_params() {
  local templates_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/template-types/${TEMPLATE_TYPE_SLUG}/templates"
  local template_id
  template_id="$(
    management_api_get "$templates_path" \
      | jq -r '.items[0].id // empty'
  )"
  if [[ -z "$template_id" ]]; then
    echo "failed to resolve template id for template type slug=${TEMPLATE_TYPE_SLUG}" >&2
    return 1
  fi

  local version_id
  version_id="$(
    management_api_get "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/templates/${template_id}/versions" \
      | jq -r '.items[] | select(.status == "published") | .id' \
      | head -n1
  )"
  if [[ -z "$version_id" ]]; then
    echo "failed to resolve published version for template_id=${template_id}" >&2
    return 1
  fi

  printf 'templateId=%s&versionId=%s' "$template_id" "$version_id"
}

current_body_text() {
  ab_json eval "$BODY_TEXT_JS" | jq -r '.data.result // ""'
}

expect_body_text() {
  local needle="$1"
  local body
  body="$(current_body_text)"
  if [[ "$body" != *"$needle"* ]]; then
    echo "expected page text to contain: $needle" >&2
    echo "actual body text: $body" >&2
    return 1
  fi
}

ensure_workspace_editor_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-template-bulk-send: loading saved browser state"
    ab state load "$STATE_FILE" >/dev/null
    return 0
  fi

  log "ui-template-bulk-send: logging in as workspace editor"
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
  ab fill "#username" "$WORKSPACE_EDITOR_EMAIL" >/dev/null
  ab fill "#password" "$WORKSPACE_EDITOR_PASSWORD" >/dev/null
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
    echo "workspace editor login did not return to frontend" >&2
    return 1
  fi

  ab state save "$STATE_FILE" >/dev/null
}

load_env_report "$ENV_REPORT_FILE"
start_frontend_dev
mailpit_clear

cat >"$INVALID_JSON_FILE" <<'EOF_INVALID'
{
  "oops": true
}
EOF_INVALID

cat >"$VALID_JSON_FILE" <<'EOF_VALID'
{
  "items": [
    {
      "to": "ui-bulk-ana@test.example.com",
      "variables": {
        "first_name": "Ana",
        "company_name": "UI Batch Corp"
      },
      "external_id": "ui-bulk-ana",
      "locale": "en"
    },
    {
      "to": "ui-bulk-beto@test.example.com",
      "variables": {
        "first_name": "Beto",
        "company_name": "UI Batch Corp"
      },
      "external_id": "ui-bulk-beto",
      "locale": "en"
    }
  ]
}
EOF_VALID

ensure_workspace_editor_login

EDITOR_QUERY="$(resolve_editor_query_params)"
TARGET_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates/${TEMPLATE_TYPE_SLUG}/edit?${EDITOR_QUERY}"
log "ui-template-bulk-send: opening $TARGET_URL"
ab open "$TARGET_URL" >/dev/null
ab wait 2500 >/dev/null

expect_body_text "Bulk Send"
expect_body_text "Send Test"
ab screenshot "$UI_SCREENSHOT_DIR/editor-before-modal.png" >/dev/null

ab find role button click --name "Bulk Send" >/dev/null
ab wait --text "Uses the current published version." >/dev/null
expect_body_text "items[] only"
expect_body_text "Use Send Test for one-off rendering checks."

if ! ab_json eval '(() => {
  const buttons = Array.from(document.querySelectorAll("button"));
  const button = buttons.find((candidate) => /confirm & queue/i.test((candidate.textContent || "").trim()));
  return button ? button.disabled : null;
})()' | jq -e '.data.result == true' >/dev/null; then
  echo "Confirm & Queue should be disabled before a valid file is uploaded" >&2
  exit 1
fi

ab upload "input[type=\"file\"]" "$INVALID_JSON_FILE" >/dev/null
ab wait --text "Fix these issues before continuing:" >/dev/null
expect_body_text "items:"
ab screenshot "$UI_SCREENSHOT_DIR/modal-invalid-json.png" >/dev/null

ab upload "input[type=\"file\"]" "$VALID_JSON_FILE" >/dev/null
ab wait --text "Preview" >/dev/null
ab wait --text "2 items" >/dev/null
expect_body_text "This will queue real emails using the current published version."
expect_body_text "ui-bulk-ana@test.example.com"
expect_body_text "ui-bulk-beto@test.example.com"
ab screenshot "$UI_SCREENSHOT_DIR/modal-valid-preview.png" >/dev/null

ab find role button click --name "Confirm & Queue" >/dev/null
ab wait --text "accepted" >/dev/null
ab wait --text "2 accepted" >/dev/null
expect_body_text "trk_"
ab screenshot "$UI_SCREENSHOT_DIR/modal-result.png" >/dev/null

mailpit_wait_for_messages 2 45
mailpit_assert_message_contains "ui-bulk-ana@test.example.com" "Ana"
mailpit_assert_message_contains "ui-bulk-beto@test.example.com" "Beto"

cat >"$UI_REPORT" <<EOF_MD
# Template Bulk Send UI Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Frontend URL: $FRONTEND_BASE_URL
- Target route: $TARGET_URL
- Actor: $WORKSPACE_EDITOR_EMAIL
- Uploaded files:
  - invalid: \`$INVALID_JSON_FILE\`
  - valid: \`$VALID_JSON_FILE\`
- Screenshots:
  - \`$UI_SCREENSHOT_DIR/editor-before-modal.png\`
  - \`$UI_SCREENSHOT_DIR/modal-invalid-json.png\`
  - \`$UI_SCREENSHOT_DIR/modal-valid-preview.png\`
  - \`$UI_SCREENSHOT_DIR/modal-result.png\`

## Validations

- Bulk Send CTA is visible in the workspace template editor.
- Invalid JSON payload shows client-side validation before submission.
- Valid \`items[]\` payload renders preview and keeps the real-send warning visible.
- Confirming the modal queues the batch and renders accepted item results with tracking IDs.
- Mailpit received both messages and the rendered bodies contain the per-item variables.
EOF_MD

log "ui-template-bulk-send: report written -> $UI_REPORT"
