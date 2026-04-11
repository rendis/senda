#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=test/system/subagents/lib.sh
source "$ROOT_DIR/test/system/subagents/lib.sh"

require_cmd jq
require_cmd curl
require_cmd docker
require_cmd corepack
require_cmd go

DEFAULT_DEMO_ARTIFACT_DIR="$ROOT_DIR/artifacts/local-code-injector-demo/$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${LOCAL_CODE_INJECTOR_DEMO_ARTIFACT_DIR:-$DEFAULT_DEMO_ARTIFACT_DIR}"
ENV_REPORT_FILE="${LOCAL_CODE_INJECTOR_DEMO_ENV_REPORT_FILE:-$ARTIFACT_DIR/env-report.json}"
RUNTIME_ENV_FILE="${LOCAL_CODE_INJECTOR_DEMO_RUNTIME_ENV_FILE:-$ARTIFACT_DIR/runtime.env}"
SUMMARY_PATH="$ARTIFACT_DIR/demo-summary.md"
FRONTEND_PID_FILE="$ARTIFACT_DIR/frontend.pid"
FRONTEND_LOG_FILE="$ARTIFACT_DIR/frontend.log"

export ARTIFACT_DIR
export ENV_REPORT_FILE
export RUNTIME_ENV_FILE

mkdir -p "$ARTIFACT_DIR"

TENANT_CODE="${TENANT_CODE:-system-test-corp}"
TENANT_NAME="${TENANT_NAME:-System Test Corp}"
WORKSPACE_CODE="${WORKSPACE_CODE:-system-main}"
WORKSPACE_NAME="${WORKSPACE_NAME:-System Main}"
INJECTOR_NAME="${INJECTOR_NAME:-student}"
TEMPLATE_TYPE_SLUG="${TEMPLATE_TYPE_SLUG:-code-injector-ui-demo}"
TEMPLATE_TYPE_NAME="${TEMPLATE_TYPE_NAME:-Code Injector UI Demo}"
RUNTIME_TEMPLATE_TYPE_SLUG="${RUNTIME_TEMPLATE_TYPE_SLUG:-code-runtime-api-demo}"
RUNTIME_TEMPLATE_TYPE_NAME="${RUNTIME_TEMPLATE_TYPE_NAME:-Code Runtime API Demo}"
ADAPTER_NAME="${ADAPTER_NAME:-Local Demo SES Adapter}"
DEFAULT_FROM_EMAIL="${DEFAULT_FROM_EMAIL:-noreply@mail.test.example.com}"

issue_test_token() {
  local email="$1"
  systemtest token --email "$email" --secret "$SENDA_E2E_JWT_SECRET" | tail -n1
}

workspace_admin_token() {
  if [[ -n "${WORKSPACE_ADMIN_TOKEN:-}" ]]; then
    printf '%s\n' "$WORKSPACE_ADMIN_TOKEN"
    return 0
  fi
  WORKSPACE_ADMIN_TOKEN="$(issue_test_token "$WORKSPACE_ADMIN_EMAIL")"
  printf '%s\n' "$WORKSPACE_ADMIN_TOKEN"
}

management_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token
  token="$(workspace_admin_token)"

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
  local response status payload

  response="$(management_api_request "$method" "$path" "$body")"
  status="$(printf '%s' "$response" | tail -n1)"
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
  local payload adapter_id

  payload="$(management_api_get "${base_path}/adapters")"
  adapter_id="$(printf '%s' "$payload" | jq -r --arg name "$ADAPTER_NAME" '.items[] | select(.name == $name) | .id' | head -n1)"

  if [[ -z "$adapter_id" ]]; then
    adapter_id="$(
      management_api_expect "201" POST "${base_path}/adapters" "$(cat <<JSON
{"adapter_type":"ses","name":"${ADAPTER_NAME}","rate_limit_per_second":100,"config":{"region":"us-east-1","access_key":"test","secret_key":"test"}}
JSON
)" | jq -r '.id // empty'
    )"
  fi

  if [[ -z "$adapter_id" ]]; then
    echo "failed to resolve demo adapter" >&2
    return 1
  fi

  upsert_default_identity "$adapter_id"
  printf '%s\n' "$adapter_id"
}

reset_student_injector_fixture() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}/injectors/${INJECTOR_NAME}"
  local status
  status="$(management_api_status DELETE "$base_path")"
  if [[ "$status" != "204" && "$status" != "404" ]]; then
    echo "failed to reset student injector fixture status=${status}" >&2
    return 1
  fi
}

ensure_student_injector_fixture() {
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}"
  reset_student_injector_fixture
  management_api_expect "201" POST "${base_path}/injectors" "$(cat <<JSON
{"name":"${INJECTOR_NAME}","description":"Demo DB injector that merges with the runtime code injector.","fields":[{"field_name":"name","field_type":"text","default_value":"Default Student","allow_overwrite":true,"position":0},{"field_name":"locked","field_type":"text","default_value":"LOCKED-DEFAULT","allow_overwrite":false,"position":1},{"field_name":"status","field_type":"text","default_value":"DEFAULT-STATUS","allow_overwrite":true,"position":2},{"field_name":"cohort","field_type":"text","default_value":"DB-COHORT","allow_overwrite":true,"position":3}]}
JSON
)" >/dev/null
}

ensure_template_fixture() {
  local slug="$1"
  local name="$2"
  local subject="$3"
  local from_name="$4"
  local preview_text="$5"
  local body_mjml="$6"
  local base_path="/api/v1/manage/tenants/${TENANT_CODE}/workspaces/${WORKSPACE_CODE}"
  local adapter_id type_status template_type_id template_id template_version_id

  adapter_id="$(ensure_workspace_adapter)"

  type_status="$(management_api_status GET "${base_path}/template-types/${slug}")"
  if [[ "$type_status" != "200" ]]; then
    management_api_expect "201" POST "${base_path}/template-types" "$(jq -nc \
      --arg slug "$slug" \
      --arg name "$name" \
      --arg adapter_id "$adapter_id" \
      '{slug:$slug,name:$name,adapter_id:$adapter_id}')" >/dev/null
  fi

  template_type_id="$(management_api_get "${base_path}/template-types/${slug}" | jq -r '.id // empty')"
  template_id="$(management_api_get "${base_path}/template-types/${slug}/templates" | jq -r '.items[0].id // empty')"
  if [[ -z "$template_id" ]]; then
    template_id="$(
      management_api_expect "201" POST "${base_path}/templates" "$(jq -nc \
        --arg template_type_id "$template_type_id" \
        '{template_type_id:$template_type_id}')" | jq -r '.id // empty'
    )"
  fi

  template_version_id="$(
    management_api_expect "201" POST "${base_path}/templates/${template_id}/versions" "$(jq -nc \
      --arg subject "$subject" \
      --arg from_name "$from_name" \
      --arg preview_text "$preview_text" \
      --arg body_mjml "$body_mjml" \
      '{subject:$subject,from_name:$from_name,preview_text:$preview_text,body_mjml:$body_mjml,default_locale:"en"}')" | jq -r '.id // empty'
  )"

  management_api_expect "204" POST "${base_path}/templates/${template_id}/versions/${template_version_id}/publish" >/dev/null || \
    management_api_expect "409" POST "${base_path}/templates/${template_id}/versions/${template_version_id}/publish" >/dev/null

  printf '%s %s %s\n' "$template_type_id" "$template_id" "$template_version_id"
}

ui_template_subject() {
  cat <<'EOF'
Brand={{ injector.brand.company_name }} | Workspace={{ injector.workspace_profile.workspace_label }} | Student={{ injector.student.name }} | Locked={{ injector.student.locked }}
EOF
}

ui_template_body() {
  cat <<'EOF'
<mjml><mj-body><mj-section><mj-column><mj-text font-size="20px">Brand={{ injector.brand.company_name }}</mj-text><mj-text>Workspace={{ injector.workspace_profile.workspace_label }} | Env={{ injector.workspace_profile.environment_name }}</mj-text><mj-image src="{{ injector.brand.hero_image_url }}" alt="Hero image" /><mj-text><a href="{{ injector.brand.policy_url }}">Policy link</a></mj-text><mj-text>{{ injector.brand.footer_html }}</mj-text><mj-text>{{ injector.workspace_profile.environment_badge_html }}</mj-text><mj-divider /><mj-text>Student={{ injector.student.name }} | Locked={{ injector.student.locked }} | Status={{ injector.student.status }} | Cohort={{ injector.student.cohort }}</mj-text><mj-text>Runtime only → ENV={{ injector.request_debug.environment }} | NOTE={{ injector.request_debug.request_note }} | EVENT={{ event.user_name }}</mj-text></mj-column></mj-section></mj-body></mjml>
EOF
}

runtime_template_subject() {
  cat <<'EOF'
Runtime={{ injector.student.name }} | Status={{ injector.student.status }} | Env={{ injector.request_debug.environment }}
EOF
}

runtime_template_body() {
  cat <<'EOF'
<mjml><mj-body><mj-section><mj-column><mj-text>NAME={{ injector.student.name }}|STATUS={{ injector.student.status }}|COHORT={{ injector.student.cohort }}|WORKSPACE={{ injector.request_debug.workspace_code }}|ENV={{ injector.request_debug.environment }}|REQUEST={{ injector.request_debug.request_note }}|EVENT={{ event.user_name }}</mj-text></mj-column></mj-section></mj-body></mjml>
EOF
}

write_summary() {
  local ui_template_id="$1"
  local ui_template_version_id="$2"
  local runtime_template_slug="$3"
  local runtime_template_id="$4"
  local runtime_template_version_id="$5"

  local ui_editor_url runtime_editor_url api_key
  ui_editor_url="${FRONTEND_BASE_URL}/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates/${TEMPLATE_TYPE_SLUG}/edit?templateId=${ui_template_id}&versionId=${ui_template_version_id}"
  runtime_editor_url="${FRONTEND_BASE_URL}/t/${TENANT_CODE}/w/${WORKSPACE_CODE}/templates/${RUNTIME_TEMPLATE_TYPE_SLUG}/edit?templateId=${runtime_template_id}&versionId=${runtime_template_version_id}"
  api_key="$(grep '^API_KEY=' "$RUNTIME_ENV_FILE" | cut -d= -f2-)"

  cat >"$SUMMARY_PATH" <<EOF
# Local Code Injector Demo

## URLs

- Frontend: ${FRONTEND_BASE_URL}
- API: ${SENDA_BASE_URL}
- Keycloak: ${KEYCLOAK_BASE_URL}
- Mailpit UI: ${MAILPIT_BASE_URL}
- UI demo editor: ${ui_editor_url}
- Runtime demo editor: ${runtime_editor_url}

## Credentials

- Superadmin: ${SUPERADMIN_EMAIL} / ${SUPERADMIN_PASSWORD}
- Workspace admin: ${WORKSPACE_ADMIN_EMAIL} / ${WORKSPACE_ADMIN_PASSWORD}

## Scope

- Tenant: ${TENANT_CODE}
- Workspace: ${WORKSPACE_CODE}
- API key: ${api_key}

## Registered code injectors

### Static / catalog-visible

- \`brand\`
  - \`company_name\` (text)
  - \`policy_url\` (url)
  - \`logo_url\` (img)
  - \`hero_image_url\` (img)
  - \`footer_html\` (html)
- \`workspace_profile\`
  - \`workspace_code\` (text)
  - \`workspace_label\` (text)
  - \`environment_name\` (text)
  - \`environment_badge_html\` (html)

### Dynamic / runtime-only

- \`student\`
  - \`name\`
  - \`status\`
  - \`cohort\`
  - \`age\`
- \`request_debug\`
  - \`workspace_code\`
  - \`environment\`
  - \`event_user_name\`
  - \`request_note\`

## DB injector fixture

- \`${INJECTOR_NAME}\`
  - \`name\` overwriteable default = \`Default Student\`
  - \`locked\` locked default = \`LOCKED-DEFAULT\`
  - \`status\` overwriteable default = \`DEFAULT-STATUS\`
  - \`cohort\` overwriteable default = \`DB-COHORT\`

## What to verify in the UI

1. Open the **UI demo editor**.
2. In the builder variable panel:
   - \`brand.*\` and \`workspace_profile.*\` should appear as **code + static**.
   - \`${INJECTOR_NAME}.*\` should appear from the DB injector.
   - \`request_debug.*\` should **not** appear because it is runtime-only.
3. In preview:
   - static code injectors and locked DB defaults should render directly;
   - overwriteable/runtime fields should remain unresolved until send.
4. In **Send Test**:
   - only overwriteable DB fields from \`${INJECTOR_NAME}\` should appear;
   - static code injectors should not appear.
5. Open Mailpit and inspect the sent email.

## Runtime send examples

### Default runtime send

\`\`\`bash
curl -X POST "${SENDA_BASE_URL}/api/v1/send" \\
  -H "Authorization: Bearer ${api_key}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "ref": "${TENANT_CODE}:${WORKSPACE_CODE}:${runtime_template_slug}",
    "to": ["runtime-default@test.example.com"],
    "variables": {"user_name": "Local Tester"}
  }'
\`\`\`

### Runtime send with injector overrides

\`\`\`bash
curl -X POST "${SENDA_BASE_URL}/api/v1/send" \\
  -H "Authorization: Bearer ${api_key}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "ref": "${TENANT_CODE}:${WORKSPACE_CODE}:${runtime_template_slug}",
    "to": ["runtime-override@test.example.com"],
    "variables": {"user_name": "Override Tester"},
    "injectors": {
      "student": {
        "name": "Ada Lovelace",
        "status": "manual-status",
        "cohort": "manual-cohort"
      },
      "request_debug": {
        "request_note": "manual-note"
      }
    }
  }'
\`\`\`

## Mailpit

- UI: ${MAILPIT_BASE_URL}
- API list: ${MAILPIT_BASE_URL}/api/v1/messages

## Stop commands

\`\`\`bash
cd ${ROOT_DIR}
bash test/system/subagents/infra-orchestrator.sh down ${ENV_REPORT_FILE}
if [[ -f ${FRONTEND_PID_FILE} ]]; then kill "\$(cat ${FRONTEND_PID_FILE})"; fi
\`\`\`
EOF
}

main() {
  bash "$ROOT_DIR/test/system/subagents/infra-orchestrator.sh" up "$ENV_REPORT_FILE"
  load_env_report

  systemtest keycloak-seed \
    --base-url "$KEYCLOAK_BASE_URL" \
    --realm "$KEYCLOAK_REALM" \
    --admin-user "$KEYCLOAK_ADMIN_USER" \
    --admin-pass "$KEYCLOAK_ADMIN_PASS" \
    --users "${NO_MEMBER_EMAIL}:${NO_MEMBER_PASSWORD}" >/dev/null || true

  systemtest seed-rbac \
    --base-url "$SENDA_BASE_URL" \
    --email "$SUPERADMIN_EMAIL" \
    --secret "$SENDA_E2E_JWT_SECRET" \
    --tenant-code "$TENANT_CODE" \
    --tenant-name "$TENANT_NAME" \
    --workspace-code "$WORKSPACE_CODE" \
    --workspace-name "$WORKSPACE_NAME" \
    --superadmin-email "$SUPERADMIN_EMAIL" \
    --tenant-admin-email "$TENANT_ADMIN_EMAIL" \
    --workspace-admin-email "$WORKSPACE_ADMIN_EMAIL" \
    --workspace-editor-email "$WORKSPACE_EDITOR_EMAIL" \
    --workspace-viewer-email "$WORKSPACE_VIEWER_EMAIL" \
    --no-member-email "$NO_MEMBER_EMAIL" >/dev/null

  start_managed_frontend "$FRONTEND_PID_FILE" "$FRONTEND_LOG_FILE" "local-code-injector-demo"

  ensure_runtime_env
  ensure_student_injector_fixture

  local ui_ids runtime_ids
  ui_ids="$(ensure_template_fixture "$TEMPLATE_TYPE_SLUG" "$TEMPLATE_TYPE_NAME" "$(ui_template_subject)" "Demo {{ injector.brand.company_name }}" "UI demo for code injectors" "$(ui_template_body)")"
  runtime_ids="$(ensure_template_fixture "$RUNTIME_TEMPLATE_TYPE_SLUG" "$RUNTIME_TEMPLATE_TYPE_NAME" "$(runtime_template_subject)" "Runtime {{ injector.student.name }}" "Runtime-only code injector demo" "$(runtime_template_body)")"

  read -r _ UI_TEMPLATE_ID UI_TEMPLATE_VERSION_ID <<<"$ui_ids"
  read -r _ RUNTIME_TEMPLATE_ID RUNTIME_TEMPLATE_VERSION_ID <<<"$runtime_ids"

  write_summary "$UI_TEMPLATE_ID" "$UI_TEMPLATE_VERSION_ID" "$RUNTIME_TEMPLATE_TYPE_SLUG" "$RUNTIME_TEMPLATE_ID" "$RUNTIME_TEMPLATE_VERSION_ID"

  printf 'artifact_dir=%s\n' "$ARTIFACT_DIR"
  printf 'summary=%s\n' "$SUMMARY_PATH"
  printf 'frontend=%s\n' "$FRONTEND_BASE_URL"
  printf 'api=%s\n' "$SENDA_BASE_URL"
  printf 'mailpit=%s\n' "$MAILPIT_BASE_URL"
}

main "$@"
