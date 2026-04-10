#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd jq
require_cmd curl
require_cmd timeout
require_cmd corepack
require_cmd docker
require_cmd go

SESSION_NAME="senda-adapter-sharing-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"
STATE_FILE="$ARTIFACT_DIR/ui-adapter-sharing.state.json"
FRONTEND_PID_FILE="$ARTIFACT_DIR/ui-adapter-sharing.frontend-dev.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/ui-adapter-sharing.frontend-dev.log"
REPORT_PATH="$ARTIFACT_DIR/ui-adapter-sharing-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-adapter-sharing"
SEND_RESPONSE_JSON="$ARTIFACT_DIR/ui-adapter-sharing-send-response.json"
DB_EVIDENCE_TSV="$ARTIFACT_DIR/ui-adapter-sharing-db.tsv"
LOG_EVIDENCE_FILE="$ARTIFACT_DIR/ui-adapter-sharing-send.log"
AUDIT_EVIDENCE_JSON="$ARTIFACT_DIR/ui-adapter-sharing-audit.json"
TEMPLATE_TYPE_JSON="$ARTIFACT_DIR/ui-adapter-sharing-template-type.json"
DASHBOARD_JSON="$ARTIFACT_DIR/ui-adapter-sharing-dashboard.json"
EMAIL_DETAIL_JSON="$ARTIFACT_DIR/ui-adapter-sharing-email-detail.json"

TENANT_CODE="${TENANT_CODE:-${SYSTEM_TENANT_CODE:-test-corp}}"
SYSTEM_WORKSPACE_CODE_UI="_system"
SYSTEM_WORKSPACE_SCOPE_LABEL_UI="Default scope"
AWS_SIM_INTERNAL_URL="${AWS_SIM_INTERNAL_URL:-http://aws-sim:4566}"
FIXTURE_SUFFIX="$(basename "$ARTIFACT_DIR" | tr '[:upper:]' '[:lower:]' | tr -cs '[:alnum:]' '-' | cut -c1-12)"
WORKSPACE_A_CODE="share-a-${FIXTURE_SUFFIX}"
WORKSPACE_A_NAME="Sharing Workspace A"
WORKSPACE_B_CODE="share-b-${FIXTURE_SUFFIX}"
WORKSPACE_B_NAME="Sharing Workspace B"
GMAIL_NAME="Shared Gmail ${FIXTURE_SUFFIX}"
SES_NAME="Shared SES ${FIXTURE_SUFFIX}"
GMAIL_DELEGATE_EMAIL="gmail-${FIXTURE_SUFFIX}@test.example.com"
SES_DOMAIN="${FIXTURE_SUFFIX}.shared-mail.test"
SES_EMAIL_A="a@${SES_DOMAIN}"
SES_EMAIL_B="b@${SES_DOMAIN}"
TEMPLATE_TYPE_SLUG="shared-ses-${FIXTURE_SUFFIX}"
TEMPLATE_TYPE_NAME="Shared SES ${FIXTURE_SUFFIX}"
WORKSPACE_B_TEMPLATE_TYPE_SLUG="shared-ses-b-${FIXTURE_SUFFIX}"
RECIPIENT_EMAIL="adapter-sharing-${FIXTURE_SUFFIX}@test.example.com"
API_KEY_NAME="adapter-sharing-${FIXTURE_SUFFIX}"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

json_string() {
  printf '%s' "$1" | jq -Rs .
}

stop_frontend_dev() {
  stop_managed_frontend "$FRONTEND_PID_FILE" "ui-adapter-sharing"
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
  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "ui-adapter-sharing"
}

tenant_admin_token() {
  if [[ -z "${TENANT_ADMIN_TOKEN:-}" ]]; then
    TENANT_ADMIN_TOKEN="$(
      systemtest token \
        --email "$TENANT_ADMIN_EMAIL" \
        --secret "$SENDA_E2E_JWT_SECRET" \
        | tail -n1
    )"
    if [[ -z "$TENANT_ADMIN_TOKEN" ]]; then
      echo "failed to issue tenant-admin test token" >&2
      return 1
    fi
  fi
  printf '%s\n' "$TENANT_ADMIN_TOKEN"
}

management_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token
  token="$(tenant_admin_token)"

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

wait_for_text() {
  local needle="$1"
  for _ in $(seq 1 60); do
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
  for _ in $(seq 1 60); do
    if ab_json eval "$expression" | jq -e '.data.result == true' >/dev/null; then
      return 0
    fi
    sleep 0.25
  done
  echo "timed out waiting for eval true: $expression" >&2
  return 1
}

ensure_tenant_admin_login() {
  if [[ -f "$STATE_FILE" ]]; then
    log "ui-adapter-sharing: loading saved browser state"
    ab state load "$STATE_FILE" >/dev/null
    return 0
  fi

  log "ui-adapter-sharing: logging in as tenant admin"
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
  ab fill "#username" "$TENANT_ADMIN_EMAIL" >/dev/null
  ab fill "#password" "$TENANT_ADMIN_PASSWORD" >/dev/null
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
    echo "tenant admin login did not return to frontend" >&2
    return 1
  fi

  ab state save "$STATE_FILE" >/dev/null
}

ensure_workspace() {
  local code="$1"
  local name="$2"
  local status
  status="$(management_api_status POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces" "{\"code\":\"${code}\",\"name\":\"${name}\"}")"
  if [[ "$status" != "201" && "$status" != "409" ]]; then
    echo "failed to ensure workspace ${code}, status=${status}" >&2
    return 1
  fi
}

create_gmail_adapter() {
  management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters" "{
    \"name\": \"${GMAIL_NAME}\",
    \"adapter_type\": \"gmail\",
    \"config\": {
      \"service_account_json\": \"{\\\"type\\\":\\\"service_account\\\",\\\"project_id\\\":\\\"system-test\\\"}\",
      \"delegate_email\": \"${GMAIL_DELEGATE_EMAIL}\"
    },
    \"is_default\": false,
    \"rate_limit_per_second\": 2
  }"
}

create_ses_adapter() {
  management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters" "{
    \"name\": \"${SES_NAME}\",
    \"adapter_type\": \"ses\",
    \"config\": {
      \"region\": \"us-east-1\",
      \"access_key_id\": \"test\",
      \"secret_access_key\": \"test\",
      \"endpoint_url\": \"${AWS_SIM_INTERNAL_URL}\"
    },
    \"is_default\": false,
    \"rate_limit_per_second\": 100
  }"
}

wait_for_provisioning() {
  local adapter_id="$1"
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    local payload
    payload="$(management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters/${adapter_id}/provisioning-status")"
    local status
    status="$(printf '%s' "$payload" | jq -r '.status // empty')"
    if [[ "$status" == "completed" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for provisioning completion for adapter=${adapter_id}" >&2
  return 1
}

ensure_tracking_provisioned() {
  local adapter_id="$1"
  management_api_expect "200" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters/${adapter_id}/auto-provision-tracking" >/dev/null
  wait_for_provisioning "$adapter_id"
}

create_aws_sim_identity() {
  local identity="$1"
  systemtest aws-sim-create-identity \
    --endpoint "$AWS_SIM_BASE_URL" \
    --identity "$identity" >/dev/null
}

list_identities() {
  local adapter_id="$1"
  management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters/${adapter_id}/identities"
}

sync_ses_identities() {
  local adapter_id="$1"
  management_api_expect "200" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters/${adapter_id}/identities/sync"
}

create_manual_identity() {
  local adapter_id="$1"
  local identity="$2"
  management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/adapters/${adapter_id}/identities" "{\"identity\":\"${identity}\"}"
}

resolve_identity_id() {
  local adapter_id="$1"
  local identity="$2"
  list_identities "$adapter_id" | jq -r --arg identity "$identity" '.[] | select(.identity == $identity) | .id' | head -n1
}

click_adapter_row_action() {
  local adapter_name="$1"
  local index="$2"
  local adapter_json
  adapter_json="$(json_string "$adapter_name")"
  if ! ab_json eval "(() => {
    const adapterName = ${adapter_json};
    const index = ${index};
    const row = Array.from(document.querySelectorAll('tbody tr')).find((candidate) =>
      (candidate.innerText || '').includes(adapterName)
    );
    if (!row) return 'missing-row';
    const actionCell = row.querySelector('td:last-child');
    if (!actionCell) return 'missing-action-cell';
    const buttons = Array.from(actionCell.querySelectorAll('button'));
    const button = buttons[index];
    if (!button) return 'missing-button';
    button.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to click action index=${index} for adapter=${adapter_name}" >&2
    return 1
  fi
}

click_identity_share_action() {
  local identity="$1"
  local identity_json
  identity_json="$(json_string "$identity")"
  if ! ab_json eval "(() => {
    const identity = ${identity_json};
    const row = Array.from(document.querySelectorAll('div')).find((candidate) => {
      const text = (candidate.textContent || '').replace(/\s+/g, ' ').trim();
      if (!text.startsWith(identity)) return false;
      return candidate.querySelectorAll('button').length > 0;
    });
    if (!row) return 'missing-row';
    const button = row.querySelector('button');
    if (!button) return 'missing-button';
    button.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to open workspace access for identity=${identity}" >&2
    return 1
  fi
}

set_workspace_dialog_selection() {
  local allowed_codes_json="$1"
  if ! ab_json eval "(() => {
    const allowed = ${allowed_codes_json};
    const labels = Array.from(document.querySelectorAll('label')).filter((candidate) => candidate.querySelector('input[type=\"checkbox\"]'));
    if (labels.length === 0) return 'missing-labels';
    for (const label of labels) {
      const text = (label.innerText || '').replace(/\s+/g, ' ').trim();
      const input = label.querySelector('input[type=\"checkbox\"]');
      const shouldCheck = allowed.some((code) => text.includes(code));
      if (!!input.checked !== shouldCheck) {
        label.click();
      }
    }
    return 'updated';
  })()" | jq -e '.data.result == "updated"' >/dev/null; then
    echo "failed to update dialog workspace selection" >&2
    return 1
  fi
}

save_dialog() {
  if ! ab_json eval '(() => {
    const button = Array.from(document.querySelectorAll("button")).find((candidate) => (candidate.textContent || "").trim() === "Save access");
    if (!button) return "missing";
    button.click();
    return "clicked";
  })()' | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to click Save access" >&2
    return 1
  fi
}

open_labeled_select() {
  local label="$1"
  local label_json
  label_json="$(json_string "$label")"
  if ! ab_json eval "(() => {
    const labelText = ${label_json};
    const section = Array.from(document.querySelectorAll('div')).find((candidate) => {
      const label = candidate.querySelector('label');
      const trigger = candidate.querySelector('[role=\"combobox\"]');
      return label && trigger && (label.textContent || '').replace(/\s+/g, ' ').trim() === labelText;
    });
    if (!section) return 'missing-section';
    const trigger = section.querySelector('[role=\"combobox\"]');
    trigger.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to open select for label=${label}" >&2
    return 1
  fi
}

choose_option_containing() {
  local needle="$1"
  local needle_json
  needle_json="$(json_string "$needle")"
  if ! ab_json eval "(() => {
    const needle = ${needle_json};
    const option = Array.from(document.querySelectorAll('[role=\"option\"]')).find((candidate) =>
      ((candidate.textContent || '').replace(/\s+/g, ' ').trim()).includes(needle)
    );
    if (!option) return 'missing-option';
    option.click();
    return 'clicked';
  })()" | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to choose option containing=${needle}" >&2
    return 1
  fi
}

assert_select_options() {
  local must_have="$1"
  local must_not_have="$2"
  local must_have_json
  local must_not_have_json
  must_have_json="$(json_string "$must_have")"
  must_not_have_json="$(json_string "$must_not_have")"
  if ! ab_json eval "(() => {
    const mustHave = ${must_have_json};
    const mustNotHave = ${must_not_have_json};
    const text = Array.from(document.querySelectorAll('[role=\"option\"]')).map((candidate) =>
      (candidate.textContent || '').replace(/\s+/g, ' ').trim()
    ).join(' | ');
    return text.includes(mustHave) && !text.includes(mustNotHave);
  })()" | jq -e '.data.result == true' >/dev/null; then
    echo "select options assertion failed: must_have=${must_have} must_not_have=${must_not_have}" >&2
    return 1
  fi
}

create_template_type_via_ui() {
  local workspace_code="$1"
  local slug="$2"
  local name="$3"
  local expected_sender="$4"
  local forbidden_sender="$5"
  local screenshot_name="$6"
  local should_submit="${7:-0}"

  ab open "$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${workspace_code}/templates" >/dev/null
  ab wait 2000 >/dev/null
  wait_for_text "New Template Type"

  if ! ab_json eval '(() => {
    const button = Array.from(document.querySelectorAll("button")).find((candidate) => (candidate.textContent || "").replace(/\s+/g, " ").trim() === "New Template Type");
    if (!button) return "missing";
    button.click();
    return "clicked";
  })()' | jq -e '.data.result == "clicked"' >/dev/null; then
    echo "failed to open New Template Type dialog" >&2
    return 1
  fi

  ab wait "#tt-name" >/dev/null
  ab fill "#tt-name" "$name" >/dev/null
  ab fill "#tt-slug" "$slug" >/dev/null

  open_labeled_select "Adapter"
  choose_option_containing "$SES_NAME"

  wait_for_eval_true "(() => Array.from(document.querySelectorAll('label')).some((candidate) => (candidate.textContent || '').trim() === 'Sender Identity'))()"
  open_labeled_select "Sender Identity"
  wait_for_eval_true "(() => document.querySelectorAll('[role=\"option\"]').length > 0)()"
  assert_select_options "$expected_sender" "$forbidden_sender"
  ab screenshot "$SCREENSHOT_DIR/${screenshot_name}" >/dev/null

  if [[ "$should_submit" == "1" ]]; then
    choose_option_containing "$expected_sender"
    if ! ab_json eval '(() => {
      const button = Array.from(document.querySelectorAll("button")).find((candidate) => (candidate.textContent || "").trim() === "Create");
      if (!button) return "missing";
      button.click();
      return "clicked";
    })()' | jq -e '.data.result == "clicked"' >/dev/null; then
      echo "failed to submit template type create dialog" >&2
      return 1
    fi
    wait_for_eval_true "(() => !document.querySelector('#tt-name'))()"
    wait_for_text "$slug"
  fi
}

resolve_template_type_id() {
  local workspace_code="$1"
  local slug="$2"
  management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${workspace_code}/template-types/${slug}" | tee "$TEMPLATE_TYPE_JSON" | jq -r '.id'
}

ensure_template_and_version() {
  local workspace_code="$1"
  local template_type_id="$2"
  local sender_email="$3"

  local template_resp template_id version_id publish_status
  template_resp="$(management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${workspace_code}/templates" "{\"template_type_id\":\"${template_type_id}\"}")"
  template_id="$(printf '%s' "$template_resp" | jq -r '.id')"

  local version_resp
  version_resp="$(management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${workspace_code}/templates/${template_id}/versions" "{
    \"subject\": \"Adapter sharing test\",
    \"preview_text\": \"Shared SES preview\",
    \"from_email\": \"${sender_email}\",
    \"from_name\": \"Sharing QA\",
    \"body_mjml\": \"<mjml><mj-body><mj-section><mj-column><mj-text>Hello {{ variable.first_name }}</mj-text></mj-column></mj-section></mj-body></mjml>\",
    \"default_locale\": \"en\"
  }")"
  version_id="$(printf '%s' "$version_resp" | jq -r '.id')"

  publish_status="$(management_api_status POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${workspace_code}/templates/${template_id}/versions/${version_id}/publish")"
  if [[ "$publish_status" != "204" ]]; then
    echo "failed to publish version ${version_id}, status=${publish_status}" >&2
    return 1
  fi
}

create_api_key() {
  local workspace_code="$1"
  management_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${workspace_code}/api-keys" "{\"name\":\"${API_KEY_NAME}\"}" | jq -r '.key // .token'
}

send_workspace_email() {
  local api_key="$1"
  local response status body
  response="$({
    curl -sS -w '\n%{http_code}' -X POST "$SENDA_BASE_URL/api/v1/send" \
      -H "Authorization: Bearer $api_key" \
      -H 'Content-Type: application/json' \
      --data "{
        \"ref\": \"${TENANT_CODE}:${WORKSPACE_A_CODE}:${TEMPLATE_TYPE_SLUG}\",
        \"to\": [\"${RECIPIENT_EMAIL}\"],
        \"variables\": {\"first_name\": \"Adapter Sharing\"}
      }"
  })"
  status="$(printf '%s' "$response" | tail -n1)"
  body="$(printf '%s' "$response" | sed '$d')"
  printf '%s\n' "$body" >"$SEND_RESPONSE_JSON"
  if [[ "$status" != "202" ]]; then
    echo "send failed: status=${status} body=${body}" >&2
    return 1
  fi
  printf '%s' "$body" | jq -r '.tracking_ids[0].tracking_id'
}

wait_for_email_status() {
  local tracking_id="$1"
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    local payload
    payload="$(management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_A_CODE}/emails/${tracking_id}")"
    printf '%s\n' "$payload" >"$EMAIL_DETAIL_JSON"
    local status
    status="$(printf '%s' "$payload" | jq -r '.status // empty')"
    local events
    events="$(printf '%s' "$payload" | jq -r '.events | length')"
    if [[ "$status" == "delivered" || "$status" == "sent" || "$events" -gt 0 ]]; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for email detail tracking_id=${tracking_id}" >&2
  return 1
}

query_db_evidence() {
  local tracking_id="$1"
  local postgres_container
  postgres_container="$(jq -r '.runtime.containers.postgres // empty' "$ENV_REPORT_FILE")"
  if [[ -z "$postgres_container" ]]; then
    echo "missing postgres container in env report" >&2
    return 1
  fi

  docker exec "$postgres_container" psql -U senda -d senda -At -F $'\t' -c "
    SELECT workspace_id::text,
           tenant_id::text,
           from_email,
           COALESCE(sender_identity_id::text, ''),
           status,
           COALESCE(provider_message_id, '')
      FROM emails
     WHERE tracking_id = '${tracking_id}'
     ORDER BY created_at DESC
     LIMIT 1;
  " >"$DB_EVIDENCE_TSV"

  if [[ ! -s "$DB_EVIDENCE_TSV" ]]; then
    echo "no email row found for tracking_id=${tracking_id}" >&2
    return 1
  fi
}

query_dashboard_evidence() {
  management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_A_CODE}/dashboard-stats" >"$DASHBOARD_JSON"
}

query_audit_evidence() {
  management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${SYSTEM_WORKSPACE_CODE_UI}/audit-log?action=update" >"$AUDIT_EVIDENCE_JSON"
}

capture_log_evidence() {
  local tracking_id="$1"
  local senda_container
  senda_container="$(jq -r '.runtime.containers.senda // empty' "$ENV_REPORT_FILE")"
  if [[ -z "$senda_container" ]]; then
    echo "missing senda container in env report" >&2
    return 1
  fi
  docker logs "$senda_container" 2>&1 | grep "$tracking_id" >"$LOG_EVIDENCE_FILE" || true
  if [[ ! -s "$LOG_EVIDENCE_FILE" ]]; then
    echo "no senda logs found for tracking_id=${tracking_id}" >&2
    return 1
  fi
}

assert_shared_row_state() {
  local adapter_name="$1"
  local expected_button_count="$2"
  local expected_disabled_json="$3"
  local adapter_json
  adapter_json="$(json_string "$adapter_name")"
  if ! ab_json eval "(() => {
    const adapterName = ${adapter_json};
    const expectedButtonCount = ${expected_button_count};
    const expectedDisabled = ${expected_disabled_json};
    const row = Array.from(document.querySelectorAll('tbody tr')).find((candidate) => (candidate.innerText || '').includes(adapterName));
    if (!row) return false;
    const actionCell = row.querySelector('td:last-child');
    if (!actionCell) return false;
    const buttons = Array.from(actionCell.querySelectorAll('button'));
    if (buttons.length !== expectedButtonCount) return false;
    return expectedDisabled.every((expected, index) => !!buttons[index].disabled === expected);
  })()" | jq -e '.data.result == true' >/dev/null; then
    echo "unexpected shared row state for adapter=${adapter_name}" >&2
    return 1
  fi
}

load_env_report "$ENV_REPORT_FILE"
ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships
start_frontend_dev
ensure_tenant_admin_login

AWS_SIM_BASE_URL="$(jq -r '.services.aws_sim // empty' "$ENV_REPORT_FILE")"
if [[ -z "$AWS_SIM_BASE_URL" ]]; then
  echo "aws_sim service URL missing from env report" >&2
  exit 1
fi

ensure_workspace "$WORKSPACE_A_CODE" "$WORKSPACE_A_NAME"
ensure_workspace "$WORKSPACE_B_CODE" "$WORKSPACE_B_NAME"

GMAIL_ADAPTER_JSON="$(create_gmail_adapter)"
GMAIL_ADAPTER_ID="$(printf '%s' "$GMAIL_ADAPTER_JSON" | jq -r '.id')"
SES_ADAPTER_JSON="$(create_ses_adapter)"
SES_ADAPTER_ID="$(printf '%s' "$SES_ADAPTER_JSON" | jq -r '.id')"

ensure_tracking_provisioned "$SES_ADAPTER_ID"
create_aws_sim_identity "$SES_DOMAIN"
sync_ses_identities "$SES_ADAPTER_ID" >/dev/null
create_manual_identity "$SES_ADAPTER_ID" "$SES_EMAIL_A" >/dev/null
create_manual_identity "$SES_ADAPTER_ID" "$SES_EMAIL_B" >/dev/null
SES_EMAIL_A_ID="$(resolve_identity_id "$SES_ADAPTER_ID" "$SES_EMAIL_A")"
SES_EMAIL_B_ID="$(resolve_identity_id "$SES_ADAPTER_ID" "$SES_EMAIL_B")"
if [[ -z "$SES_EMAIL_A_ID" || -z "$SES_EMAIL_B_ID" ]]; then
  echo "failed to resolve SES email identity ids" >&2
  exit 1
fi

SYSTEM_ADAPTERS_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${SYSTEM_WORKSPACE_CODE_UI}/adapters"
log "ui-adapter-sharing: opening $SYSTEM_ADAPTERS_URL"
ab open "$SYSTEM_ADAPTERS_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "$GMAIL_NAME"
wait_for_text "$SES_NAME"
ab screenshot "$SCREENSHOT_DIR/01-system-adapters.png" >/dev/null

click_adapter_row_action "$GMAIL_NAME" 0
wait_for_text "Workspace access — ${GMAIL_NAME}"
set_workspace_dialog_selection "[\"${WORKSPACE_A_CODE}\"]"
ab screenshot "$SCREENSHOT_DIR/02-gmail-workspace-access.png" >/dev/null
save_dialog
wait_for_eval_true "(() => !document.body?.innerText?.includes('Workspace access — ${GMAIL_NAME}') )()"

click_adapter_row_action "$SES_NAME" 0
wait_for_text "Sender Identities — ${SES_NAME}"
wait_for_text "$SES_DOMAIN"
wait_for_text "$SES_EMAIL_A"
wait_for_text "$SES_EMAIL_B"
ab screenshot "$SCREENSHOT_DIR/03-ses-identity-panel.png" >/dev/null

click_identity_share_action "$SES_EMAIL_A"
wait_for_text "Workspace access — ${SES_EMAIL_A}"
set_workspace_dialog_selection "[\"${WORKSPACE_A_CODE}\"]"
ab screenshot "$SCREENSHOT_DIR/04-ses-email-a-access.png" >/dev/null
save_dialog
wait_for_eval_true "(() => !document.body?.innerText?.includes('Workspace access — ${SES_EMAIL_A}') )()"

click_identity_share_action "$SES_EMAIL_B"
wait_for_text "Workspace access — ${SES_EMAIL_B}"
set_workspace_dialog_selection "[\"${WORKSPACE_B_CODE}\"]"
ab screenshot "$SCREENSHOT_DIR/05-ses-email-b-access.png" >/dev/null
save_dialog
wait_for_eval_true "(() => !document.body?.innerText?.includes('Workspace access — ${SES_EMAIL_B}') )()"

WORKSPACE_A_ADAPTERS_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_A_CODE}/adapters"
log "ui-adapter-sharing: opening $WORKSPACE_A_ADAPTERS_URL"
ab open "$WORKSPACE_A_ADAPTERS_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "$GMAIL_NAME"
wait_for_text "$SES_NAME"
wait_for_text "Shared"
assert_shared_row_state "$GMAIL_NAME" 3 '[true,false,true]'
assert_shared_row_state "$SES_NAME" 4 '[false,true,false,true]'
ab screenshot "$SCREENSHOT_DIR/06-workspace-a-shared-adapters.png" >/dev/null

click_adapter_row_action "$SES_NAME" 0
wait_for_text "Sender Identities — ${SES_NAME}"
wait_for_text "Shared from ${SYSTEM_WORKSPACE_SCOPE_LABEL_UI} — read only"
wait_for_text "$SES_EMAIL_A"
if ab_json eval "(() => (document.body?.innerText || '').includes('${SES_EMAIL_B}'))()" | jq -e '.data.result == true' >/dev/null; then
  echo "workspace A shared sender panel should not expose ${SES_EMAIL_B}" >&2
  exit 1
fi
ab screenshot "$SCREENSHOT_DIR/07-workspace-a-shared-senders-read-only.png" >/dev/null

create_template_type_via_ui "$WORKSPACE_A_CODE" "$TEMPLATE_TYPE_SLUG" "$TEMPLATE_TYPE_NAME" "$SES_EMAIL_A" "$SES_EMAIL_B" "08-workspace-a-sender-filter.png" 1
create_template_type_via_ui "$WORKSPACE_B_CODE" "$WORKSPACE_B_TEMPLATE_TYPE_SLUG" "Workspace B Shared SES ${FIXTURE_SUFFIX}" "$SES_EMAIL_B" "$SES_EMAIL_A" "09-workspace-b-sender-filter.png" 0

WORKSPACE_B_ADAPTERS_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_B_CODE}/adapters"
log "ui-adapter-sharing: opening $WORKSPACE_B_ADAPTERS_URL"
ab open "$WORKSPACE_B_ADAPTERS_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "$SES_NAME"
if ab_json eval "(() => (document.body?.innerText || '').includes('${GMAIL_NAME}'))()" | jq -e '.data.result == true' >/dev/null; then
  echo "workspace B should not see gmail adapter ${GMAIL_NAME}" >&2
  exit 1
fi
ab screenshot "$SCREENSHOT_DIR/10-workspace-b-adapters-isolation.png" >/dev/null

TEMPLATE_TYPE_ID="$(resolve_template_type_id "$WORKSPACE_A_CODE" "$TEMPLATE_TYPE_SLUG")"
if [[ -z "$TEMPLATE_TYPE_ID" ]]; then
  echo "failed to resolve template type id for ${TEMPLATE_TYPE_SLUG}" >&2
  exit 1
fi
if [[ "$(jq -r '.sender_identity_id // empty' "$TEMPLATE_TYPE_JSON")" != "$SES_EMAIL_A_ID" ]]; then
  echo "template type sender_identity_id mismatch" >&2
  exit 1
fi

ensure_template_and_version "$WORKSPACE_A_CODE" "$TEMPLATE_TYPE_ID" "$SES_EMAIL_A"
API_KEY_VALUE="$(create_api_key "$WORKSPACE_A_CODE")"
if [[ -z "$API_KEY_VALUE" ]]; then
  echo "failed to create API key for workspace A" >&2
  exit 1
fi

TRACKING_ID="$(send_workspace_email "$API_KEY_VALUE")"
if [[ -z "$TRACKING_ID" ]]; then
  echo "failed to resolve tracking id from send response" >&2
  exit 1
fi

wait_for_email_status "$TRACKING_ID"
query_db_evidence "$TRACKING_ID"
query_dashboard_evidence
query_audit_evidence
capture_log_evidence "$TRACKING_ID"

DB_WORKSPACE_ID="$(cut -f1 "$DB_EVIDENCE_TSV")"
DB_TENANT_ID="$(cut -f2 "$DB_EVIDENCE_TSV")"
DB_FROM_EMAIL="$(cut -f3 "$DB_EVIDENCE_TSV")"
DB_SENDER_IDENTITY_ID="$(cut -f4 "$DB_EVIDENCE_TSV")"
DB_STATUS="$(cut -f5 "$DB_EVIDENCE_TSV")"
DB_PROVIDER_MESSAGE_ID="$(cut -f6 "$DB_EVIDENCE_TSV")"
WORKSPACE_A_ID="$(management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_A_CODE}" | jq -r '.id')"
TENANT_ID="$(management_api_expect "200" GET "/api/v1/manage/tenants/${TENANT_CODE}" | jq -r '.id')"

if [[ "$DB_WORKSPACE_ID" != "$WORKSPACE_A_ID" ]]; then
  echo "workspace_id evidence mismatch: expected=${WORKSPACE_A_ID} actual=${DB_WORKSPACE_ID}" >&2
  exit 1
fi
if [[ "$DB_TENANT_ID" != "$TENANT_ID" ]]; then
  echo "tenant_id evidence mismatch: expected=${TENANT_ID} actual=${DB_TENANT_ID}" >&2
  exit 1
fi
if [[ "$DB_FROM_EMAIL" != "$SES_EMAIL_A" ]]; then
  echo "from_email evidence mismatch: expected=${SES_EMAIL_A} actual=${DB_FROM_EMAIL}" >&2
  exit 1
fi
if [[ "$DB_SENDER_IDENTITY_ID" != "$SES_EMAIL_A_ID" ]]; then
  echo "sender_identity_id evidence mismatch: expected=${SES_EMAIL_A_ID} actual=${DB_SENDER_IDENTITY_ID}" >&2
  exit 1
fi
if [[ -z "$DB_PROVIDER_MESSAGE_ID" ]]; then
  echo "expected provider message id to be persisted" >&2
  exit 1
fi

if ! jq -e --arg sender_id "$SES_EMAIL_A_ID" --arg from_email "$SES_EMAIL_A" '.by_adapter[] | select(.sender_identity_id == $sender_id and .from_email == $from_email and .totals.sent >= 1)' "$DASHBOARD_JSON" >/dev/null; then
  echo "dashboard evidence missing shared SES breakdown row" >&2
  exit 1
fi
if ! jq -e '.items | map(select(.entity_type == "adapter" or .entity_type == "adapter_identity")) | length >= 2' "$AUDIT_EVIDENCE_JSON" >/dev/null; then
  echo "audit evidence missing adapter sharing updates" >&2
  exit 1
fi
if ! grep -q "$SES_EMAIL_A_ID" "$LOG_EVIDENCE_FILE"; then
  echo "expected sender_identity_id in senda logs" >&2
  exit 1
fi
if ! grep -q "$TRACKING_ID" "$LOG_EVIDENCE_FILE"; then
  echo "expected tracking_id in senda logs" >&2
  exit 1
fi

WORKSPACE_A_EMAIL_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_A_CODE}/emails/${TRACKING_ID}"
log "ui-adapter-sharing: opening $WORKSPACE_A_EMAIL_URL"
ab open "$WORKSPACE_A_EMAIL_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "Shipping Information"
wait_for_text "$SES_EMAIL_A"
ab screenshot "$SCREENSHOT_DIR/11-workspace-a-email-detail.png" >/dev/null

WORKSPACE_A_DASHBOARD_URL="$FRONTEND_BASE_URL/t/${TENANT_CODE}/w/${WORKSPACE_A_CODE}"
log "ui-adapter-sharing: opening $WORKSPACE_A_DASHBOARD_URL"
ab open "$WORKSPACE_A_DASHBOARD_URL" >/dev/null
ab wait 2500 >/dev/null
wait_for_text "By Provider"
wait_for_text "$SES_EMAIL_A"
ab screenshot "$SCREENSHOT_DIR/12-workspace-a-dashboard.png" >/dev/null

cat >"$REPORT_PATH" <<EOF_MD
# UI Adapter Sharing Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Tenant: ${TENANT_CODE}
- System workspace: ${SYSTEM_WORKSPACE_CODE_UI}
- Workspace A: ${WORKSPACE_A_CODE}
- Workspace B: ${WORKSPACE_B_CODE}
- Gmail adapter: ${GMAIL_NAME} (${GMAIL_ADAPTER_ID})
- SES adapter: ${SES_NAME} (${SES_ADAPTER_ID})
- SES domain: ${SES_DOMAIN}
- SES sender A: ${SES_EMAIL_A} (${SES_EMAIL_A_ID})
- SES sender B: ${SES_EMAIL_B} (${SES_EMAIL_B_ID})
- Template type A: ${TEMPLATE_TYPE_SLUG} (${TEMPLATE_TYPE_ID})
- Tracking ID: ${TRACKING_ID}
- DB status: ${DB_STATUS}
- Provider message ID: ${DB_PROVIDER_MESSAGE_ID}

## Verified Flow

1. Tenant admin logged in and opened the _system adapters UI.
2. Gmail shared-access dialog granted only workspace A.
3. SES identity panel showed the synced domain plus both manual sender emails.
4. SES sender access dialogs granted:
   - ${SES_EMAIL_A} -> ${WORKSPACE_A_CODE}
   - ${SES_EMAIL_B} -> ${WORKSPACE_B_CODE}
5. Workspace A adapters page showed Gmail + SES as shared/read-only.
6. Workspace A shared SES sender panel exposed only ${SES_EMAIL_A}.
7. Workspace A template type dialog exposed ${SES_EMAIL_A} and excluded ${SES_EMAIL_B}.
8. Workspace B template type dialog exposed ${SES_EMAIL_B} and excluded ${SES_EMAIL_A}.
9. Workspace B adapters page did not expose the shared Gmail adapter.
10. A real send from workspace A persisted:
    - workspace_id = ${DB_WORKSPACE_ID}
    - tenant_id = ${DB_TENANT_ID}
    - from_email = ${DB_FROM_EMAIL}
    - sender_identity_id = ${DB_SENDER_IDENTITY_ID}
11. Dashboard breakdown contains the shared SES row split by sender identity/from_email.
12. Audit log contains workspace-access updates for adapter + identity sharing.
13. Structured logs contain both tracking_id and sender_identity_id.

## Artifacts

- Screenshots: 
  - ${SCREENSHOT_DIR}/01-system-adapters.png
  - ${SCREENSHOT_DIR}/02-gmail-workspace-access.png
  - ${SCREENSHOT_DIR}/03-ses-identity-panel.png
  - ${SCREENSHOT_DIR}/04-ses-email-a-access.png
  - ${SCREENSHOT_DIR}/05-ses-email-b-access.png
  - ${SCREENSHOT_DIR}/06-workspace-a-shared-adapters.png
  - ${SCREENSHOT_DIR}/07-workspace-a-shared-senders-read-only.png
  - ${SCREENSHOT_DIR}/08-workspace-a-sender-filter.png
  - ${SCREENSHOT_DIR}/09-workspace-b-sender-filter.png
  - ${SCREENSHOT_DIR}/10-workspace-b-adapters-isolation.png
  - ${SCREENSHOT_DIR}/11-workspace-a-email-detail.png
  - ${SCREENSHOT_DIR}/12-workspace-a-dashboard.png
- Send response: ${SEND_RESPONSE_JSON}
- Template type evidence: ${TEMPLATE_TYPE_JSON}
- Email detail evidence: ${EMAIL_DETAIL_JSON}
- Dashboard evidence: ${DASHBOARD_JSON}
- Audit evidence: ${AUDIT_EVIDENCE_JSON}
- DB evidence: ${DB_EVIDENCE_TSV}
- Log evidence: ${LOG_EVIDENCE_FILE}
EOF_MD

log "ui-adapter-sharing: report written -> $REPORT_PATH"
