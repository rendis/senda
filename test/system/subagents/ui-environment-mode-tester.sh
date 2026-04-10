#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd corepack

SESSION_NAME="senda-environment-mode-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-environment-mode.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/ui-environment-mode.frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/ui-environment-mode.frontend-dev.log"
REPORT_PATH="$ARTIFACT_DIR/ui-environment-mode-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-environment-mode"

TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
FIXTURE_SUFFIX="$(basename "$ARTIFACT_DIR" | tr '[:upper:]' '[:lower:]' | tr -cs '[:alnum:]' '-' | rev | cut -c1-12 | rev | sed 's/^-*//; s/-*$//')"
WORKSPACE_CODE="${WORKSPACE_CODE:-${UI_ENV_WORKSPACE_CODE:-ui-env-${FIXTURE_SUFFIX}}}"
WORKSPACE_NAME="${WORKSPACE_NAME:-${UI_ENV_WORKSPACE_NAME:-UI Environment Fixture}}"
TARGET_TENANT_CODE="$TENANT_CODE"
TARGET_WORKSPACE_CODE="$WORKSPACE_CODE"
TARGET_WORKSPACE_NAME="$WORKSPACE_NAME"
TENANT_WORKSPACES_SCOPE_CODE="${TENANT_WORKSPACES_SCOPE_CODE:-_system}"
TEMPLATE_TYPE_SLUG="${UI_ENV_TEMPLATE_TYPE_SLUG:-env-mode-${FIXTURE_SUFFIX}}"
TEMPLATE_TYPE_NAME="${UI_ENV_TEMPLATE_TYPE_NAME:-Environment Mode Fixture}"
WORKSPACE_TEST_RECIPIENTS="${UI_ENV_WORKSPACE_RECIPIENTS:-env-workspace-${FIXTURE_SUFFIX}@test.example.com
env-workspace-review-${FIXTURE_SUFFIX}@test.example.com}"
TEMPLATE_TEST_RECIPIENTS="${UI_ENV_TEMPLATE_RECIPIENTS:-env-template-${FIXTURE_SUFFIX}@test.example.com
env-template-review-${FIXTURE_SUFFIX}@test.example.com}"
WORKSPACE_TEST_RECIPIENT_A="${WORKSPACE_TEST_RECIPIENTS%%$'\n'*}"
WORKSPACE_TEST_RECIPIENT_B="${WORKSPACE_TEST_RECIPIENTS#*$'\n'}"
TEMPLATE_TEST_RECIPIENT_A="${TEMPLATE_TEST_RECIPIENTS%%$'\n'*}"
TEMPLATE_TEST_RECIPIENT_B="${TEMPLATE_TEST_RECIPIENTS#*$'\n'}"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

stop_frontend_dev() {
  stop_managed_frontend "$FRONTEND_PID_FILE" "ui-environment-mode"
}

cleanup() {
  timeout 5s agent-browser --session "$SESSION_NAME" close >/dev/null 2>&1 || true
  stop_frontend_dev
}

trap cleanup EXIT

start_frontend_dev() {
  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "ui-environment-mode"
}

management_api_token() {
  if [[ -z "${MANAGEMENT_API_TOKEN:-}" ]]; then
    MANAGEMENT_API_TOKEN="$(
      systemtest token \
        --email "$SUPERADMIN_EMAIL" \
        --secret "$SENDA_E2E_JWT_SECRET" \
        | tail -n1
    )"
    if [[ -z "$MANAGEMENT_API_TOKEN" ]]; then
      echo "failed to issue superadmin test token" >&2
      return 1
    fi
  fi
  printf '%s\n' "$MANAGEMENT_API_TOKEN"
}

management_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token
  token="$(management_api_token)"

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

ensure_workspace_fixture() {
  local status
  status="$(management_api_status POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces" "$(cat <<EOF_JSON
{"code":"${WORKSPACE_CODE}","name":"${WORKSPACE_NAME}"}
EOF_JSON
)")"

  if [[ "$status" != "201" && "$status" != "409" ]]; then
    echo "failed to ensure workspace fixture code=${WORKSPACE_CODE} status=${status}" >&2
    return 1
  fi
}

ensure_template_type_fixture() {
  local base_path="/api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/template-types"

  local status
  status="$(management_api_status GET "${base_path}/${TEMPLATE_TYPE_SLUG}")"
  if [[ "$status" == "200" ]]; then
    return 0
  fi

  management_api_expect "201" POST "${base_path}" "$(cat <<EOF_JSON
{"slug":"${TEMPLATE_TYPE_SLUG}","name":"${TEMPLATE_TYPE_NAME}"}
EOF_JSON
)" >/dev/null
}

ensure_management_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-environment-mode: discarding stale saved browser state"
    rm -f "$STATE_FILE"
  fi

  log "ui-environment-mode: logging in as superadmin"
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
  ab fill "#username" "$SUPERADMIN_EMAIL" >/dev/null
  ab fill "#password" "$SUPERADMIN_PASSWORD" >/dev/null
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
    echo "superadmin login did not return to frontend" >&2
    return 1
  fi

  ab state save "$STATE_FILE" >/dev/null
}

wait_for_text() {
  local needle="$1"
  for _ in $(seq 1 80); do
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
  for _ in $(seq 1 120); do
    if ab_json eval "$expression" | jq -e '.data.result == true' >/dev/null; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for eval true: $expression" >&2
  return 1
}

wait_for_url() {
  local expected="$1"
  for _ in $(seq 1 80); do
    local current
    current="$(ab_json get url | jq -r '.data.url // ""')"
    if [[ "$current" == "$expected" ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for url: $expected" >&2
  return 1
}

click_sidebar_link() {
  local label="$1"
  local label_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const expected = ${label_json};
    const links = Array.from(document.querySelectorAll('nav a'));
    const target = links.find((candidate) => {
      const text = (candidate.textContent || '').replace(/\\s+/g, ' ').trim();
      return text === expected;
    });
    if (!target) return 'missing';
    target.click();
    return target.getAttribute('href') || 'clicked';
  })()" | jq -e '.data.result != "missing"' >/dev/null; then
    echo "sidebar link not found: ${label}" >&2
    return 1
  fi
}

click_row_by_code() {
  local code="$1"
  local code_json
  code_json="$(printf '%s' "$code" | jq -Rs .)"
  if ! ab_json eval "(() => {
    const code = ${code_json};
    const row = Array.from(document.querySelectorAll('tbody tr')).find((candidate) =>
      (candidate.innerText || '').includes(code)
    );
    if (!row) return 'missing';
    row.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "table row not found for code=${code}" >&2
    return 1
  fi
}

click_button_by_aria_label() {
  local label="$1"
  local label_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"

  wait_for_eval_true "(() => {
    const label = ${label_json};
    return Array.from(document.querySelectorAll('button')).some((candidate) => candidate.getAttribute('aria-label') === label);
  })()"

  if ab find role button click --name "$label" >/dev/null 2>&1; then
    return 0
  fi

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

select_combobox_option_by_label() {
  local label="$1"
  local option="$2"
  local label_json option_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"
  option_json="$(printf '%s' "$option" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const expectedLabel = ${label_json};
    const labels = Array.from(document.querySelectorAll('label'));
    const label = labels.find((candidate) => (candidate.textContent || '').replace(/\\s+/g, ' ').trim() === expectedLabel);
    if (!label) return 'missing-label';
    let container = label.parentElement;
    while (container && !container.querySelector('[role=\"combobox\"]')) {
      container = container.parentElement;
    }
    const trigger = container?.querySelector('[role=\"combobox\"]');
    if (!trigger) return 'missing-trigger';
    trigger.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "combobox trigger not found for label=${label}" >&2
    return 1
  fi

  if ! ab_json eval "(() => {
    const expectedOption = ${option_json};
    const options = Array.from(document.querySelectorAll('[role=\"option\"]'));
    const target = options.find((candidate) => (candidate.textContent || '').replace(/\\s+/g, ' ').trim() === expectedOption);
    if (!target) return 'missing-option';
    target.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "combobox option not found for label=${label} option=${option}" >&2
    return 1
  fi
}

fill_textarea_by_label() {
  local label="$1"
  local value="$2"
  local label_json value_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"
  value_json="$(printf '%s' "$value" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const expectedLabel = ${label_json};
    const nextValue = ${value_json};
    const labels = Array.from(document.querySelectorAll('label'));
    const label = labels.find((candidate) => (candidate.textContent || '').replace(/\\s+/g, ' ').trim() === expectedLabel);
    if (!label) return 'missing-label';
    let container = label.parentElement;
    while (container && !container.querySelector('textarea')) {
      container = container.parentElement;
    }
    const textarea = container?.querySelector('textarea');
    if (!textarea) return 'missing-textarea';
    textarea.focus();
    textarea.value = nextValue;
    textarea.dispatchEvent(new Event('input', { bubbles: true }));
    textarea.dispatchEvent(new Event('change', { bubbles: true }));
    return 'filled';
  })()" | jq -e '.data.result == "filled"' >/dev/null; then
    echo "textarea not found for label=${label}" >&2
    return 1
  fi
}

set_environment_mode() {
  local mode="$1"
  local button_text
  button_text="$(printf '%s' "$mode" | tr '[:lower:]' '[:upper:]')"
  local button_json
  button_json="$(printf '%s' "$button_text" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const expected = ${button_json};
    const button = Array.from(document.querySelectorAll('header button[aria-pressed]')).find((candidate) =>
      (candidate.textContent || '').replace(/\\s+/g, ' ').trim() === expected
    );
    if (!button) return 'missing';
    button.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "environment toggle button not found for mode=${mode}" >&2
    return 1
  fi
}

assert_header_environment_state() {
  local expected="$1"
  local class_fragment="$2"
  local expected_button
  expected_button="$(printf '%s' "$expected" | tr '[:lower:]' '[:upper:]')"
  local expected_json class_json button_json
  expected_json="$(printf '%s' "$expected" | jq -Rs .)"
  class_json="$(printf '%s' "$class_fragment" | jq -Rs .)"
  button_json="$(printf '%s' "$expected_button" | jq -Rs .)"

  wait_for_eval_true "(() => {
    const expected = ${expected_json};
    const classFragment = ${class_json};
    const expectedButton = ${button_json};
    const active = document.querySelector('header button[aria-pressed=\"true\"]');
    if (!active) return false;
    const chip = active.closest('div')?.previousElementSibling;
    if (!chip) return false;
    const activeText = (active.textContent || '').replace(/\\s+/g, ' ').trim();
    const chipText = (chip.textContent || '').replace(/\\s+/g, ' ').trim().toLowerCase();
    const chipClass = chip.className || '';
    return activeText === expectedButton && chipText === expected && chipClass.includes(classFragment);
  })()"
}

assert_page_environment_badge() {
  local expected="$1"
  local class_fragment="$2"
  local expected_json class_json
  expected_json="$(printf '%s' "$expected" | jq -Rs .)"
  class_json="$(printf '%s' "$class_fragment" | jq -Rs .)"

  wait_for_eval_true "(() => {
    const expected = ${expected_json};
    const classFragment = ${class_json};
    const badges = Array.from(document.querySelectorAll('span')).filter((candidate) => {
      const text = (candidate.textContent || '').replace(/\\s+/g, ' ').trim().toLowerCase();
      return text === expected && (candidate.className || '').includes('rounded-full');
    });
    return badges.some((candidate) => (candidate.className || '').includes(classFragment));
  })()"
}

assert_prod_workspace_dialog_hides_test_controls() {
  wait_for_eval_true "(() => {
    const body = (document.body?.innerText || '').replace(/\\s+/g, ' ').trim();
    const hasRecipients = body.includes('Safe recipients');
    const hasMode = body.includes('Test recipient mode');
    const hasWarning = body.includes('Test mode never sends to real recipients directly');
    const textarea = document.querySelector('#edit-workspace-test-recipients');
    return !hasRecipients && !hasMode && !hasWarning && !textarea;
  })()"
}

assert_prod_workspaces_page_hides_test_controls() {
  wait_for_eval_true "(() => {
    const body = (document.body?.innerText || '').replace(/\\s+/g, ' ').trim();
    return !body.includes('Safe recipients')
      && !body.includes('Test recipient mode')
      && !body.includes('Test mode never sends to real recipients directly');
  })()"
}

assert_test_workspace_dialog_shows_test_controls() {
  wait_for_eval_true "(() => {
    const body = (document.body?.innerText || '').replace(/\\s+/g, ' ').trim();
    const hasRecipients = body.includes('Safe recipients');
    const hasMode = body.includes('Test recipient mode');
    const hasWarning = body.includes('Test mode never sends to real recipients directly');
    const textarea = document.querySelector('#edit-workspace-test-recipients');
    return hasRecipients && hasMode && hasWarning && !!textarea;
  })()"
}

assert_template_type_dialog_shows_override_controls() {
  wait_for_eval_true "(() => {
    const body = (document.body?.innerText || '').replace(/\\s+/g, ' ').trim();
    const hasOverride = body.includes('Test recipient override');
    const hasMode = body.includes('Override mode');
    return hasOverride && hasMode;
  })()"
}

load_env_report "$ENV_REPORT_FILE"
start_frontend_dev
seed_keycloak_users
seed_rbac_memberships
TENANT_CODE="$TARGET_TENANT_CODE"
WORKSPACE_CODE="$TARGET_WORKSPACE_CODE"
WORKSPACE_NAME="$TARGET_WORKSPACE_NAME"
ensure_workspace_fixture
ensure_template_type_fixture
ensure_management_login

WORKSPACE_ROOT_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}"
WORKSPACE_TEST_URL="${WORKSPACE_ROOT_URL}?environment=test"
TEMPLATES_TEST_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates?environment=test"
SETTINGS_TEST_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/settings?environment=test"
WORKSPACES_PROD_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${TENANT_WORKSPACES_SCOPE_CODE}/workspaces"
WORKSPACES_TEST_URL="${WORKSPACES_PROD_URL}?environment=test"
WORKSPACE_TEST_API_PATH="/api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}"
TEMPLATE_TYPE_TEST_API_PATH="/api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/template-types/${TEMPLATE_TYPE_SLUG}"

log "ui-environment-mode: opening workspace dashboard in prod"
ab open "$WORKSPACE_ROOT_URL" >/dev/null
wait_for_text "Dashboard"
assert_header_environment_state "prod" "bg-emerald"
ab screenshot "$SCREENSHOT_DIR/01-workspace-prod-dashboard.png" >/dev/null

log "ui-environment-mode: toggling workspace header to test"
set_environment_mode "test"
wait_for_url "$WORKSPACE_TEST_URL"
assert_header_environment_state "test" "bg-amber"
ab screenshot "$SCREENSHOT_DIR/02-workspace-test-dashboard.png" >/dev/null

log "ui-environment-mode: asserting environment persists to templates navigation"
click_sidebar_link "Templates"
wait_for_url "$TEMPLATES_TEST_URL"
wait_for_text "Templates"
assert_header_environment_state "test" "bg-amber"
ab screenshot "$SCREENSHOT_DIR/03-templates-test-navigation.png" >/dev/null

log "ui-environment-mode: asserting environment persists to settings navigation"
click_sidebar_link "Settings"
wait_for_url "$SETTINGS_TEST_URL"
wait_for_text "Settings"
assert_header_environment_state "test" "bg-amber"
ab screenshot "$SCREENSHOT_DIR/04-settings-test-navigation.png" >/dev/null

log "ui-environment-mode: validating workspace environment-scoped API state"
WORKSPACE_PROD_RESPONSE="$(management_api_expect "200" GET "/api/v1/manage/environments/prod/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}")"
printf '%s\n' "$WORKSPACE_PROD_RESPONSE" | jq -e \
  '.environment == "prod"
   and ((.test_recipient_addresses // []) | length == 0)
   and ((.test_recipient_mode // "replace") == "replace")' >/dev/null

WORKSPACE_TEST_RESPONSE_PREPARED="$(management_api_expect "200" GET "$WORKSPACE_TEST_API_PATH")"
printf '%s\n' "$WORKSPACE_TEST_RESPONSE_PREPARED" | jq -e \
  '.environment == "test"
   and ((.test_recipient_addresses // []) | length == 0)
   and ((.test_recipient_mode // "replace") == "replace")' >/dev/null

management_api_expect "200" PUT "$WORKSPACE_TEST_API_PATH" "$(cat <<EOF_JSON
{"test_recipient_mode":"append","test_recipient_addresses":["${WORKSPACE_TEST_RECIPIENT_A}","${WORKSPACE_TEST_RECIPIENT_B}"]}
EOF_JSON
)" >/dev/null

WORKSPACE_TEST_RESPONSE="$(management_api_expect "200" GET "$WORKSPACE_TEST_API_PATH")"
printf '%s\n' "$WORKSPACE_TEST_RESPONSE" | jq -e \
  --arg addr1 "$WORKSPACE_TEST_RECIPIENT_A" \
  --arg addr2 "$WORKSPACE_TEST_RECIPIENT_B" \
  '.environment == "test"
   and .test_recipient_mode == "append"
   and (.test_recipient_addresses | index($addr1))
   and (.test_recipient_addresses | index($addr2))' >/dev/null
ab screenshot "$SCREENSHOT_DIR/05-workspace-api-state.png" >/dev/null || true

log "ui-environment-mode: validating template-type test override UI"
ab open "$TEMPLATES_TEST_URL" >/dev/null
wait_for_url "$TEMPLATES_TEST_URL"
wait_for_text "Templates"
wait_for_text "$TEMPLATE_TYPE_SLUG"
click_button_by_aria_label "Edit template type ${TEMPLATE_TYPE_SLUG}"
wait_for_text "Change the name, slug, adapter, and sender assigned to this template type."
assert_template_type_dialog_shows_override_controls
select_combobox_option_by_label "Override mode" "Append safe recipients"
wait_for_text "Warning: append preserves original recipients and adds these addresses."
fill_textarea_by_label "Safe recipients" "$TEMPLATE_TEST_RECIPIENTS"
ab screenshot "$SCREENSHOT_DIR/08-template-type-test-override.png" >/dev/null

cat >"$REPORT_PATH" <<EOF
# UI Environment Mode Report

- Route set: workspace dashboard, templates, settings, tenant workspace list
- Actor: \`${SUPERADMIN_EMAIL}\`
- Workspace under test: \`${WORKSPACE_CODE}\`
- Workspace fixture name: \`${WORKSPACE_NAME}\`
- Template type fixture: \`${TEMPLATE_TYPE_SLUG}\`

## Covered

- Workspace header renders the prod/test toggle and the active environment indicator with the expected visual color family.
- Switching from prod to test updates the workspace URL to \`?environment=test\`.
- Sidebar navigation preserves \`environment=test\` when moving from dashboard to templates and settings.
- Workspace environment state is validated through env-scoped management API reads and updates.
- Test template-type edit dialog shows test-recipient override controls; selecting append surfaces the warning copy and reveals the safe-recipient textarea in UI.

## Artifacts

- \`$SCREENSHOT_DIR/01-workspace-prod-dashboard.png\`
- \`$SCREENSHOT_DIR/02-workspace-test-dashboard.png\`
- \`$SCREENSHOT_DIR/03-templates-test-navigation.png\`
- \`$SCREENSHOT_DIR/04-settings-test-navigation.png\`
- \`$SCREENSHOT_DIR/05-workspace-api-state.png\`
- \`$SCREENSHOT_DIR/08-template-type-test-override.png\`
EOF

log "ui-environment-mode: report written -> $REPORT_PATH"
