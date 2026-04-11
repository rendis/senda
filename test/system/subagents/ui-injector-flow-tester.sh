#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd corepack

BODY_TEXT_JS='(() => (document.body?.innerText || "").replace(/\s+/g, " ").trim())()'

SESSION_NAME="senda-injector-flow-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-injector-flow.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/ui-injector-flow.frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/ui-injector-flow.frontend-dev.log"
REPORT_PATH="$ARTIFACT_DIR/ui-injector-flow-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-injector-flow"
VALID_BULK_JSON_FILE="$SCREENSHOT_DIR/valid-injector-bulk-send.json"

TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
WORKSPACE_CODE="${WORKSPACE_CODE:-${SYSTEM_WORKSPACE_CODE:-main}}"
FIXTURE_SUFFIX="${FIXTURE_SUFFIX:-$(date +%s)}"
TEMPLATE_TYPE_SLUG="${TEMPLATE_TYPE_SLUG:-injector-ui-${FIXTURE_SUFFIX}}"
TEMPLATE_TYPE_NAME="${TEMPLATE_TYPE_NAME:-Injector UI Fixture}"
INJECTOR_NAME="${INJECTOR_NAME:-student}"
ADAPTER_NAME="${ADAPTER_NAME:-Injector UI Adapter ${FIXTURE_SUFFIX}}"
DEFAULT_FROM_EMAIL="${DEFAULT_FROM_EMAIL:-noreply@mail.test.example.com}"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

stop_frontend_dev() {
  stop_managed_frontend "$FRONTEND_PID_FILE" "ui-injector-flow"
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
  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "ui-injector-flow"
}

issue_test_token() {
  local email="$1"
  systemtest token \
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
      echo "failed to obtain workspace-admin token" >&2
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

management_api_get() {
  management_api_expect "200" GET "$1"
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

assert_create_button_disabled() {
  if ! ab_json eval '(() => {
    const buttons = Array.from(document.querySelectorAll("button"));
    const button = buttons.find((candidate) => /create/i.test((candidate.textContent || "").trim()));
    return button ? button.disabled : null;
  })()' | jq -e '.data.result == true' >/dev/null; then
    echo "expected Create button to be disabled" >&2
    return 1
  fi
}

assert_input_disabled() {
  local selector="$1"
  if ! ab_json eval "(() => { const el = document.querySelector('${selector}'); return el ? !!el.disabled : null; })()" | jq -e '.data.result == true' >/dev/null; then
    echo "expected input ${selector} to be disabled" >&2
    return 1
  fi
}

assert_input_value() {
  local selector="$1"
  local expected="$2"
  if ! ab_json eval "(() => { const el = document.querySelector('${selector}'); return el ? ('value' in el ? el.value : null) : null; })()" | jq -e --arg expected "$expected" '.data.result == $expected' >/dev/null; then
    echo "expected ${selector} value=${expected}" >&2
    return 1
  fi
}

assert_visible() {
  local selector="$1"
  if ! ab_json eval "(() => { const el = document.querySelector('${selector}'); return !!(el && el.offsetParent !== null); })()" | jq -e '.data.result == true' >/dev/null; then
    echo "expected ${selector} to be visible" >&2
    return 1
  fi
}

assert_not_present() {
  local selector="$1"
  if ! ab_json eval "(() => document.querySelector('${selector}') === null)()" | jq -e '.data.result == true' >/dev/null; then
    echo "expected ${selector} to be absent" >&2
    return 1
  fi
}

click_selector() {
  local selector="$1"
  if ! ab_json eval "(() => {
    const el = document.querySelector('${selector}');
    if (!el) return 'missing';
    el.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to click selector=${selector}" >&2
    return 1
  fi
}

click_button_by_text() {
  local label="$1"
  local exact="${2:-1}"
  local label_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"
  if ! ab_json eval "(() => {
    const label = ${label_json};
    const exact = ${exact};
    const normalize = (value) => (value || '').replace(/\\s+/g, ' ').trim();
    const buttons = Array.from(document.querySelectorAll('button'));
    const target = buttons.find((candidate) => {
      const text = normalize(candidate.textContent);
      return exact ? text === label : text.includes(label);
    });
    if (!target) return 'missing';
    target.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to click button with label=${label}" >&2
    return 1
  fi
}

set_input_value() {
  local selector="$1"
  local value="$2"
  local value_json
  value_json="$(printf '%s' "$value" | jq -Rs .)"
  if ! ab_json eval "(() => {
    const el = document.querySelector('${selector}');
    if (!el) return 'missing';
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set;
    if (!setter) return 'missing-setter';
    setter.call(el, ${value_json});
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    return 'ok';
  })()" | jq -e '.data.result == "ok"' >/dev/null; then
    echo "failed to set value for selector=${selector}" >&2
    return 1
  fi
}

set_overwrite_state() {
  local allow_selector="$1"
  local locked_selector="$2"
  local expected="$3"
  local selector="$allow_selector"
  if [[ "$expected" == "false" ]]; then
    selector="$locked_selector"
  fi
  ab eval "(() => {
    const el = document.querySelector('${selector}');
    if (!el) return 'missing';
    el.click();
    return 'clicked';
  })()" >/dev/null
}

ensure_workspace_admin_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-injector-flow: loading saved browser state"
    ab state load "$STATE_FILE" >/dev/null
    return 0
  fi

  log "ui-injector-flow: logging in as workspace admin"
  ab open "$FRONTEND_BASE_URL/login" >/dev/null
  ab wait 1200 >/dev/null

  if ! ab_json eval '(() => {
    const buttons = Array.from(document.querySelectorAll("button"));
    const button = buttons.find((candidate) => /sign in|oidc|iniciar|ingresar/i.test((candidate.textContent || "").replace(/\s+/g, " ").trim()));
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

upsert_default_identity() {
  local adapter_id="$1"
  local postgres_container
  postgres_container="$(jq -r '.runtime.containers.postgres // empty' "$ENV_REPORT_FILE")"
  if [[ -z "$postgres_container" ]]; then
    echo "missing postgres container in env report" >&2
    return 1
  fi

  docker exec "$postgres_container" psql -U senda -d senda -v ON_ERROR_STOP=1 -c "
    UPDATE adapter_identities
       SET is_default = false,
           updated_at = NOW()
     WHERE adapter_id = '${adapter_id}'::uuid
       AND identity <> '${DEFAULT_FROM_EMAIL}';

    INSERT INTO adapter_identities (
      id, adapter_id, identity, identity_type, status,
      sending_enabled, is_default, source, last_synced_at, created_at, updated_at
    )
    VALUES (
      gen_random_uuid(), '${adapter_id}'::uuid, '${DEFAULT_FROM_EMAIL}', 'email', 'verified',
      true, true, 'provider', NOW(), NOW(), NOW()
    )
    ON CONFLICT (adapter_id, identity) DO UPDATE
      SET status = 'verified',
          sending_enabled = true,
          is_default = true,
          source = 'provider',
          last_synced_at = NOW(),
          updated_at = NOW();
  " >/dev/null
}

ensure_workspace_adapter() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}"
  local payload
  payload="$(management_api_get "${base_path}/adapters")"
  local adapter_id
  adapter_id="$(printf '%s' "$payload" | jq -r --arg name "$ADAPTER_NAME" '.items[] | select(.name == $name) | .id' | head -n1)"

  if [[ -z "$adapter_id" ]]; then
    adapter_id="$(
      management_api_expect "201" POST "${base_path}/adapters" "$(cat <<EOF_JSON
{"adapter_type":"ses","name":"${ADAPTER_NAME}","rate_limit_per_second":100,"config":{"region":"us-east-1","access_key":"test","secret_key":"test"}}
EOF_JSON
)" | jq -r '.id // empty'
    )"
  fi

  if [[ -z "$adapter_id" ]]; then
    echo "failed to resolve workspace adapter" >&2
    return 1
  fi

  upsert_default_identity "$adapter_id"
  printf '%s\n' "$adapter_id"
}

ensure_template_fixture() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}"
  local adapter_id
  adapter_id="$(ensure_workspace_adapter)"

  local type_status
  type_status="$(management_api_status GET "${base_path}/template-types/${TEMPLATE_TYPE_SLUG}")"
  if [[ "$type_status" != "200" ]]; then
    management_api_expect "201" POST "${base_path}/template-types" "{\"slug\":\"${TEMPLATE_TYPE_SLUG}\",\"name\":\"${TEMPLATE_TYPE_NAME}\",\"adapter_id\":\"${adapter_id}\"}" >/dev/null
  fi

  TEMPLATE_TYPE_ID="$(
    management_api_get "${base_path}/template-types/${TEMPLATE_TYPE_SLUG}" \
      | jq -r '.id // empty'
  )"
  if [[ -z "$TEMPLATE_TYPE_ID" ]]; then
    echo "failed to resolve template type id" >&2
    return 1
  fi

  TEMPLATE_ID="$(
    management_api_get "${base_path}/template-types/${TEMPLATE_TYPE_SLUG}/templates" \
      | jq -r '.items[0].id // empty'
  )"
  if [[ -z "$TEMPLATE_ID" ]]; then
    TEMPLATE_ID="$(
      management_api_expect "201" POST "${base_path}/templates" "{\"template_type_id\":\"${TEMPLATE_TYPE_ID}\"}" \
        | jq -r '.id // empty'
    )"
  fi
  if [[ -z "$TEMPLATE_ID" ]]; then
    echo "failed to resolve template id" >&2
    return 1
  fi

  TEMPLATE_VERSION_ID="$(
    management_api_expect "201" POST "${base_path}/templates/${TEMPLATE_ID}/versions" "$(cat <<EOF_JSON
{"subject":"NAME={{ injector.${INJECTOR_NAME}.name }}|LOCKED={{ injector.${INJECTOR_NAME}.locked }}|STATUS={{ injector.${INJECTOR_NAME}.status }}","preview_text":"Injector UI preview","from_name":"UI Fixture","body_mjml":"<mjml><mj-body><mj-section><mj-column><mj-text>NAME={{ injector.${INJECTOR_NAME}.name }}|LOCKED={{ injector.${INJECTOR_NAME}.locked }}|STATUS={{ injector.${INJECTOR_NAME}.status }}|EVENT={{ event.user_name }}</mj-text></mj-column></mj-section></mj-body></mjml>","default_locale":"en"}
EOF_JSON
)" | jq -r '.id // empty'
  )"
  if [[ -z "$TEMPLATE_VERSION_ID" ]]; then
    echo "failed to create template version" >&2
    return 1
  fi

  management_api_expect "204" POST "${base_path}/templates/${TEMPLATE_ID}/versions/${TEMPLATE_VERSION_ID}/publish" >/dev/null || \
    management_api_expect "409" POST "${base_path}/templates/${TEMPLATE_ID}/versions/${TEMPLATE_VERSION_ID}/publish" >/dev/null
}

reset_injector_fixture() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/injectors/${INJECTOR_NAME}"
  local status
  status="$(management_api_status DELETE "$base_path")"
  if [[ "$status" != "204" && "$status" != "404" ]]; then
    echo "failed to reset injector fixture status=${status}" >&2
    return 1
  fi
}

resolve_editor_query_params() {
  printf 'templateId=%s&versionId=%s' "$TEMPLATE_ID" "$TEMPLATE_VERSION_ID"
}

create_injector_via_ui() {
  local target_url="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/injectors"
  log "ui-injector-flow: opening ${target_url}"
  ab open "$target_url" >/dev/null
  ab wait 2000 >/dev/null
  if ! expect_body_text "New Injector"; then
    expect_body_text "Add Injector"
  fi

  log "ui-injector-flow: opening injector dialog"
  if ! click_selector '[data-testid="injector-create-trigger"]'; then
    click_selector '[data-testid="injector-empty-create-trigger"]'
  fi
  ab wait "#injector-name" >/dev/null
  assert_visible '[data-testid="injector-field-header-0"]'

  ab fill "#injector-name" "$INJECTOR_NAME" >/dev/null
  ab fill '[data-testid="injector-field-name-0"]' "name" >/dev/null
  set_overwrite_state '[data-testid="injector-field-overwrite-0-allow"]' '[data-testid="injector-field-overwrite-0-locked"]' false
  assert_visible '[data-testid="injector-field-default-error-0"]'
  assert_create_button_disabled

  ab fill '[data-testid="injector-field-default-0"]' "Default Student" >/dev/null
  set_overwrite_state '[data-testid="injector-field-overwrite-0-allow"]' '[data-testid="injector-field-overwrite-0-locked"]' true

  click_button_by_text "Add Field"
  assert_visible '[data-testid="injector-field-header-1"]'
  ab fill '[data-testid="injector-field-name-1"]' "locked" >/dev/null
  ab fill '[data-testid="injector-field-default-1"]' "LOCKED-DEFAULT" >/dev/null
  set_overwrite_state '[data-testid="injector-field-overwrite-1-allow"]' '[data-testid="injector-field-overwrite-1-locked"]' false

  click_button_by_text "Add Field"
  assert_visible '[data-testid="injector-field-header-2"]'
  ab fill '[data-testid="injector-field-name-2"]' "status" >/dev/null
  ab fill '[data-testid="injector-field-default-2"]' "DEFAULT-STATUS" >/dev/null

  log "ui-injector-flow: submitting injector form"
  click_button_by_text "Create"
  ab wait 2000 >/dev/null
  expect_body_text "$INJECTOR_NAME"
  expect_body_text "3 fields"
}

verify_injector_builder_and_test_send() {
  local editor_query
  editor_query="$(resolve_editor_query_params)"
  local target_url="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates/${TEMPLATE_TYPE_SLUG}/edit?${editor_query}"

  log "ui-injector-flow: opening template editor ${target_url}"
  ab open "$target_url" >/dev/null
  ab wait 2500 >/dev/null

  expect_body_text "Send Test"
  expect_body_text "Bulk Send"
  expect_body_text "${INJECTOR_NAME}.name"
  expect_body_text "${INJECTOR_NAME}.locked"
  expect_body_text "${INJECTOR_NAME}.status"
  ab screenshot "$SCREENSHOT_DIR/template-editor.png" >/dev/null

  log "ui-injector-flow: running default test send"
  click_button_by_text "Send Test"
  ab wait --text "Send Test Email" >/dev/null

  assert_input_value "[data-testid=\"test-send-field-${INJECTOR_NAME}-name\"]" "Default Student"
  assert_input_value "[data-testid=\"test-send-field-${INJECTOR_NAME}-status\"]" "DEFAULT-STATUS"
  assert_not_present "[data-testid=\"test-send-field-${INJECTOR_NAME}-locked\"]"
  ab screenshot "$SCREENSHOT_DIR/test-send-modal-defaults.png" >/dev/null

  mailpit_clear
  ab fill '[data-testid="test-send-email"]' "ui-injector-default@test.example.com" >/dev/null
  ab fill '[data-testid="test-send-variables-json"]' '{"user_name":"UIDefault"}' >/dev/null
  click_selector '[data-testid="test-send-submit"]'
  mailpit_wait_for_messages 1 30
  mailpit_assert_message_contains "ui-injector-default@test.example.com" "NAME=Code Student|LOCKED=LOCKED-DEFAULT|STATUS=code-status|EVENT=UIDefault"

  log "ui-injector-flow: running override test send"
  click_button_by_text "Send Test"
  ab wait --text "Send Test Email" >/dev/null
  mailpit_clear
  ab fill '[data-testid="test-send-email"]' "ui-injector-override@test.example.com" >/dev/null
  ab fill '[data-testid="test-send-variables-json"]' '{"user_name":"UIOverride"}' >/dev/null
  ab fill "[data-testid=\"test-send-field-${INJECTOR_NAME}-name\"]" "UI Override" >/dev/null
  click_selector '[data-testid="test-send-submit"]'
  mailpit_wait_for_messages 1 30
  mailpit_assert_message_contains "ui-injector-override@test.example.com" "NAME=UI Override|LOCKED=LOCKED-DEFAULT|STATUS=code-status|EVENT=UIOverride"

  log "ui-injector-flow: running empty override test send"
  click_button_by_text "Send Test"
  ab wait --text "Send Test Email" >/dev/null
  mailpit_clear
  ab fill '[data-testid="test-send-email"]' "ui-injector-empty@test.example.com" >/dev/null
  ab fill '[data-testid="test-send-variables-json"]' '{"user_name":"UIEmpty"}' >/dev/null
  set_input_value "[data-testid=\"test-send-field-${INJECTOR_NAME}-name\"]" "tmp"
  set_input_value "[data-testid=\"test-send-field-${INJECTOR_NAME}-name\"]" ""
  assert_input_value "[data-testid=\"test-send-field-${INJECTOR_NAME}-name\"]" ""
  click_selector '[data-testid="test-send-submit"]'
  mailpit_wait_for_messages 1 30
  mailpit_assert_message_contains "ui-injector-empty@test.example.com" "NAME=|LOCKED=LOCKED-DEFAULT|STATUS=code-status|EVENT=UIEmpty"
}

verify_bulk_send_ui() {
  log "ui-injector-flow: preparing bulk send fixture"
  cat >"$VALID_BULK_JSON_FILE" <<EOF_JSON
{
  "items": [
    {
      "to": "ui-injector-bulk-one@test.example.com",
      "variables": { "user_name": "BulkOne" },
      "injectors": {
        "${INJECTOR_NAME}": { "name": "Bulk One" }
      },
      "external_id": "ui-injector-bulk-one",
      "locale": "en"
    },
    {
      "to": "ui-injector-bulk-two@test.example.com",
      "variables": { "user_name": "BulkTwo" },
      "injectors": {
        "${INJECTOR_NAME}": { "status": "bulk-two-status" }
      },
      "external_id": "ui-injector-bulk-two",
      "locale": "en"
    }
  ]
}
EOF_JSON

  log "ui-injector-flow: opening bulk send dialog"
  click_button_by_text "Bulk Send"
  ab wait --text "Uses the current published version." >/dev/null
  ab upload 'input[type="file"]' "$VALID_BULK_JSON_FILE" >/dev/null
  ab wait --text "Preview" >/dev/null
  expect_body_text "Injectors 1"
  ab screenshot "$SCREENSHOT_DIR/bulk-send-preview.png" >/dev/null

  mailpit_clear
  log "ui-injector-flow: submitting bulk send"
  click_button_by_text "Confirm & Queue"
  ab wait --text "accepted" >/dev/null
  ab screenshot "$SCREENSHOT_DIR/bulk-send-result.png" >/dev/null

  mailpit_wait_for_messages 2 45
  mailpit_assert_message_contains "ui-injector-bulk-one@test.example.com" "NAME=Bulk One|LOCKED=LOCKED-DEFAULT|STATUS=code-status|EVENT=BulkOne"
  mailpit_assert_message_contains "ui-injector-bulk-two@test.example.com" "NAME=Code Student|LOCKED=LOCKED-DEFAULT|STATUS=bulk-two-status|EVENT=BulkTwo"
}

ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships
start_frontend_dev
ensure_workspace_admin_login
ensure_template_fixture
reset_injector_fixture
create_injector_via_ui
verify_injector_builder_and_test_send
verify_bulk_send_ui

cat >"$REPORT_PATH" <<EOF_MD
# Injector UI Flow Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Frontend URL: $FRONTEND_BASE_URL
- Tenant/workspace: ${TENANT_CODE}/${WORKSPACE_CODE}
- Template slug: ${TEMPLATE_TYPE_SLUG}
- Injector: ${INJECTOR_NAME}
- Screenshots:
  - \`$SCREENSHOT_DIR/template-editor.png\`
  - \`$SCREENSHOT_DIR/test-send-modal-defaults.png\`
  - \`$SCREENSHOT_DIR/bulk-send-preview.png\`
  - \`$SCREENSHOT_DIR/bulk-send-result.png\`

## Validations

- Workspace admin can create the injector catalog from the UI.
- Locked fields block creation until a non-empty default is provided.
- The template editor exposes workspace injector tokens in the builder.
- Test Send recognizes only overwriteable injector fields, hides locked/static fields, and resolves default/code/request precedence correctly.
- Bulk Send accepts per-item injector overrides from UI-uploaded JSON and renders each item with the expected precedence.
- Mailpit confirms rendered content for the default, partial override, explicit empty override, and bulk-send scenarios.
EOF_MD

log "ui-injector-flow: report written -> $REPORT_PATH"
