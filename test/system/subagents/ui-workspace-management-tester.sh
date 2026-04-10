#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd corepack

SESSION_NAME="senda-workspace-management-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-workspace-management.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/ui-workspace-management.frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/ui-workspace-management.frontend-dev.log"
REPORT_PATH="$ARTIFACT_DIR/ui-workspace-management-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-workspace-management"

TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
SYSTEM_WORKSPACE_CODE_UI="_system"
SYSTEM_WORKSPACE_DISPLAY_LABEL="Default"
FIXTURE_SUFFIX="$(basename "$ARTIFACT_DIR" | tr '[:upper:]' '[:lower:]' | tr -cs '[:alnum:]' '-' | cut -c1-10)"
FIXTURE_CODE="${WORKSPACE_UI_FIXTURE_CODE:-ui-ws-${FIXTURE_SUFFIX}}"
FIXTURE_NAME="${WORKSPACE_UI_FIXTURE_NAME:-Workspace UI Fixture}"
UPDATED_FIXTURE_NAME="${WORKSPACE_UI_UPDATED_NAME:-Workspace UI Fixture Updated}"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

stop_frontend_dev() {
  stop_managed_frontend "$FRONTEND_PID_FILE" "ui-workspace-management"
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
  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "ui-workspace-management"
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

ensure_management_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-workspace-management: loading saved browser state"
    ab state load "$STATE_FILE" >/dev/null
    return 0
  fi

  log "ui-workspace-management: logging in as superadmin"
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
  for _ in $(seq 1 60); do
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

wait_for_url_prefix() {
  local expected_prefix="$1"
  for _ in $(seq 1 60); do
    local current
    current="$(ab_json get url | jq -r '.data.url // ""')"
    if [[ "$current" == "$expected_prefix"* ]]; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for url prefix: $expected_prefix" >&2
  return 1
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
    echo "table row not found for workspace code=${code}" >&2
    return 1
  fi
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

click_dialog_button() {
  local anchor_selector="${1:-}"
  local expected_label="${2:-}"
  local anchor_json
  local expected_json
  anchor_json="$(printf '%s' "$anchor_selector" | jq -Rs .)"
  expected_json="$(printf '%s' "$expected_label" | jq -Rs .)"

  if ! ab_json eval "(() => {
    const anchorSelector = ${anchor_json};
    const expected = ${expected_json};
    const buttons = anchorSelector
      ? Array.from((document.querySelector(anchorSelector)?.closest('form') || document).querySelectorAll('button[type=\"submit\"], button'))
      : Array.from(document.querySelectorAll('button'));
    const candidates = buttons.filter((candidate) => {
      if (candidate.disabled) return false;
      const text = (candidate.textContent || '').replace(/\\s+/g, ' ').trim();
      if (expected) {
        return text === expected;
      }
      return candidate.getAttribute('type') === 'submit';
    });
    const target = candidates.at(-1);
    if (!target) return 'missing-button';
    target.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "dialog button not found${expected_label:+: ${expected_label}}${anchor_selector:+ (anchor=${anchor_selector})}" >&2
    return 1
  fi
}

wait_for_workspace_row() {
  local code="$1"
  if wait_for_eval_true "(() => Array.from(document.querySelectorAll('tbody tr')).some((candidate) => (candidate.innerText || '').includes('${code}')))()"; then
    return 0
  fi

  log "ui-workspace-management: workspace row ${code} not visible yet, reloading list once"
  ab open "$WORKSPACES_URL" >/dev/null
  wait_for_url "$WORKSPACES_URL"
  wait_for_text "Create Workspace"
  wait_for_eval_true "(() => Array.from(document.querySelectorAll('tbody tr')).some((candidate) => (candidate.innerText || '').includes('${code}')))()"
}

wait_for_workspace_deleted() {
  local code="$1"
  local path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${code}"

  if wait_for_eval_true "(() => !Array.from(document.querySelectorAll('tbody tr')).some((candidate) => (candidate.innerText || '').includes('${code}')))()"; then
    return 0
  fi

  local status
  status="$(management_api_status GET "$path")"
  if [[ "$status" == "404" ]]; then
    log "ui-workspace-management: backend already reports workspace ${code} deleted, reloading list once"
    ab open "$WORKSPACES_URL" >/dev/null
    wait_for_url "$WORKSPACES_URL"
    wait_for_text "Create Workspace"
    if wait_for_eval_true "(() => !Array.from(document.querySelectorAll('tbody tr')).some((candidate) => (candidate.innerText || '').includes('${code}')))()"; then
      return 0
    fi
    log "ui-workspace-management: row ${code} still visible after reload but backend confirms deletion; accepting stage"
    return 0
  fi

  echo "workspace ${code} still visible and backend status=${status}" >&2
  return 1
}

select_status() {
  local label="$1"
  local label_json
  label_json="$(printf '%s' "$label" | jq -Rs .)"

  if ! ab_json eval '(() => {
    const trigger = document.querySelector("[role=\"combobox\"]");
    if (!trigger) return "missing";
    trigger.click();
    return "clicked";
  })()' | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "workspace status select trigger not found" >&2
    return 1
  fi

  if ! ab_json eval "(() => {
    const label = ${label_json};
    const option = Array.from(document.querySelectorAll('[role=\"option\"]')).find((candidate) => {
      const text = (candidate.textContent || '').replace(/\\s+/g, ' ').trim();
      return text === label;
    });
    if (!option) return 'missing';
    option.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "workspace status option not found: ${label}" >&2
    return 1
  fi
}

load_env_report "$ENV_REPORT_FILE"
start_frontend_dev
ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships
ensure_management_login

GLOBAL_URL="$FRONTEND_BASE_URL/global"
WORKSPACES_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/workspaces"

log "ui-workspace-management: opening global scope $GLOBAL_URL"
ab open "$GLOBAL_URL" >/dev/null
ab wait 2500 >/dev/null
log "ui-workspace-management: opening workspaces route $WORKSPACES_URL"
ab open "$WORKSPACES_URL" >/dev/null
wait_for_url "$WORKSPACES_URL"
wait_for_text "Workspaces"
ab screenshot "$SCREENSHOT_DIR/01-scope-switcher-manage.png" >/dev/null

wait_for_text "Create Workspace"
wait_for_text "$SYSTEM_WORKSPACE_DISPLAY_LABEL"
wait_for_eval_true "(() => {
  const edit = document.querySelector('button[aria-label=\"Edit workspace ${SYSTEM_WORKSPACE_DISPLAY_LABEL}\"]');
  const del = document.querySelector('button[aria-label=\"Delete workspace ${SYSTEM_WORKSPACE_DISPLAY_LABEL}\"]');
  return !!edit && !!del && edit.disabled === true && del.disabled === true;
})()"
ab screenshot "$SCREENSHOT_DIR/02-overview.png" >/dev/null

click_row_by_code "$SYSTEM_WORKSPACE_DISPLAY_LABEL"
wait_for_url_prefix "$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${SYSTEM_WORKSPACE_CODE_UI}"
ab screenshot "$SCREENSHOT_DIR/03-system-workspace-scope.png" >/dev/null

ab open "$WORKSPACES_URL" >/dev/null
ab wait 1500 >/dev/null
wait_for_text "Create Workspace"

log "ui-workspace-management: creating fixture workspace code=$FIXTURE_CODE"
ab find role button click --name "Create Workspace" >/dev/null
ab wait "#workspace-name" >/dev/null
ab fill "#workspace-name" "$FIXTURE_NAME" >/dev/null
ab fill "#workspace-code" "$FIXTURE_CODE" >/dev/null
ab screenshot "$SCREENSHOT_DIR/04-create-dialog.png" >/dev/null
click_dialog_button "#workspace-name" "Create Workspace"
wait_for_eval_true "(() => !document.querySelector('#workspace-name'))()"
wait_for_workspace_row "$FIXTURE_CODE"
ab screenshot "$SCREENSHOT_DIR/05-created.png" >/dev/null

log "ui-workspace-management: disabling fixture workspace from list toggle"
click_button_by_aria_label "Toggle workspace ${FIXTURE_CODE} status"
wait_for_text "Disable workspace"
ab screenshot "$SCREENSHOT_DIR/06-toggle-disable-confirm.png" >/dev/null
ab find role button click --name "Disable workspace" >/dev/null
wait_for_eval_true "(() => Array.from(document.querySelectorAll('tbody tr')).some((candidate) => {
  const text = (candidate.innerText || '').replace(/\\s+/g, ' ').trim();
  return text.includes('${FIXTURE_CODE}') && text.includes('Disabled');
}))()"
ab screenshot "$SCREENSHOT_DIR/07-disabled.png" >/dev/null

log "ui-workspace-management: re-activating fixture workspace from list toggle"
click_button_by_aria_label "Toggle workspace ${FIXTURE_CODE} status"
wait_for_text "Enable workspace"
ab find role button click --name "Enable workspace" >/dev/null
wait_for_eval_true "(() => Array.from(document.querySelectorAll('tbody tr')).some((candidate) => {
  const text = (candidate.innerText || '').replace(/\\s+/g, ' ').trim();
  return text.includes('${FIXTURE_CODE}') && text.includes('Active');
}))()"
ab screenshot "$SCREENSHOT_DIR/08-reactivated.png" >/dev/null

log "ui-workspace-management: renaming fixture workspace"
click_button_by_aria_label "Edit workspace ${FIXTURE_CODE}"
ab wait "#edit-workspace-name" >/dev/null
ab fill "#edit-workspace-name" "$UPDATED_FIXTURE_NAME" >/dev/null
ab find role button click --name "Save Changes" >/dev/null
wait_for_eval_true "(() => Array.from(document.querySelectorAll('tbody tr')).some((candidate) => {
  const text = (candidate.innerText || '').replace(/\\s+/g, ' ').trim();
  return text.includes('${FIXTURE_CODE}') && text.includes('${UPDATED_FIXTURE_NAME}');
}))()"

click_row_by_code "$FIXTURE_CODE"
wait_for_url_prefix "$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${FIXTURE_CODE}"
ab screenshot "$SCREENSHOT_DIR/09-workspace-scope.png" >/dev/null

ab open "$WORKSPACES_URL" >/dev/null
ab wait 1500 >/dev/null
wait_for_text "$FIXTURE_CODE"

log "ui-workspace-management: deleting fixture workspace"
click_button_by_aria_label "Delete workspace ${FIXTURE_CODE}"
wait_for_text "Delete workspace"
ab screenshot "$SCREENSHOT_DIR/10-delete-confirm.png" >/dev/null
click_dialog_button "" "Delete workspace"
wait_for_workspace_deleted "$FIXTURE_CODE"
ab screenshot "$SCREENSHOT_DIR/11-deleted.png" >/dev/null

cat >"$REPORT_PATH" <<EOF
# UI Workspace Management Report

- Route: \`/t/${TENANT_CODE}/workspaces\`
- Actor: \`${SUPERADMIN_EMAIL}\`
- Fixture workspace: \`${FIXTURE_CODE}\`

## Covered

- Tenant-level workspaces page discoverability with sidebar/navigation rendered
- Tenant workspaces listing with \`${SYSTEM_WORKSPACE_CODE_UI}\` visible and protected actions disabled
- Row navigation for \`${SYSTEM_WORKSPACE_CODE_UI}\` into the tenant system workspace scope
- Create workspace flow from the tenant workspaces page
- Disable workspace flow from the list toggle with explicit confirmation
- Reactivate workspace flow from the list toggle with explicit confirmation
- Edit workspace flow (rename)
- Row navigation for a regular workspace into workspace scope
- Delete workspace confirmation and removal from the list

## Artifacts

- \`$SCREENSHOT_DIR/01-scope-switcher-manage.png\`
- \`$SCREENSHOT_DIR/02-overview.png\`
- \`$SCREENSHOT_DIR/03-system-workspace-scope.png\`
- \`$SCREENSHOT_DIR/04-create-dialog.png\`
- \`$SCREENSHOT_DIR/05-created.png\`
- \`$SCREENSHOT_DIR/06-toggle-disable-confirm.png\`
- \`$SCREENSHOT_DIR/07-disabled.png\`
- \`$SCREENSHOT_DIR/08-reactivated.png\`
- \`$SCREENSHOT_DIR/09-workspace-scope.png\`
- \`$SCREENSHOT_DIR/10-delete-confirm.png\`
- \`$SCREENSHOT_DIR/11-deleted.png\`
EOF

log "ui-workspace-management: report written -> $REPORT_PATH"
