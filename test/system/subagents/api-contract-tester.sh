#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd go
require_cmd make
require_cmd node
require_cmd jq
require_cmd corepack

load_env_report "$ENV_REPORT_FILE"

REPORT_PATH="$ARTIFACT_DIR/api-contract-report.md"
RUNTIME_ENV_FILE="${RUNTIME_ENV_FILE:-$ARTIFACT_DIR/runtime.env}"
COLLECTION="$ROOT_DIR/docs/postman/senda-api-v1.postman_collection.json"
ENV_FILE="$ROOT_DIR/docs/postman/senda-local.postman_environment.json"
JSON_REPORT="$ARTIFACT_DIR/newman-report.json"
JUNIT_REPORT="$ARTIFACT_DIR/newman-junit.xml"
AWS_SIM_INTERNAL_URL="${AWS_SIM_INTERNAL_URL:-http://aws-sim:4566}"

SHARED_WORKSPACE_A_CODE="${SHARED_WORKSPACE_A_CODE:-api-share-a}"
SHARED_WORKSPACE_A_NAME="${SHARED_WORKSPACE_A_NAME:-API Share Workspace A}"
SHARED_WORKSPACE_B_CODE="${SHARED_WORKSPACE_B_CODE:-api-share-b}"
SHARED_WORKSPACE_B_NAME="${SHARED_WORKSPACE_B_NAME:-API Share Workspace B}"
SHARED_GMAIL_NAME="${SHARED_GMAIL_NAME:-API Contract Shared Gmail}"
SHARED_GMAIL_DELEGATE_EMAIL="${SHARED_GMAIL_DELEGATE_EMAIL:-api-contract-shared@test.example.com}"
SHARED_SES_NAME="${SHARED_SES_NAME:-API Contract Shared SES}"
SHARED_SES_DOMAIN="${SHARED_SES_DOMAIN:-api-contract.shared-mail.test}"
SHARED_SES_DEFAULT_EMAIL="${SHARED_SES_DEFAULT_EMAIL:-default@${SHARED_SES_DOMAIN}}"
SHARED_SES_TEMP_EMAIL="${SHARED_SES_TEMP_EMAIL:-shared-sender@${SHARED_SES_DOMAIN}}"
ENV_FIXTURE_SUFFIX="$(basename "$ARTIFACT_DIR" | tr '[:upper:]' '[:lower:]' | tr -cs '[:alnum:]' '-' | cut -c1-10)"
ENV_FIXTURE_WORKSPACE_CODE="${ENV_FIXTURE_WORKSPACE_CODE:-api-env-${ENV_FIXTURE_SUFFIX}}"
ENV_FIXTURE_WORKSPACE_NAME="${ENV_FIXTURE_WORKSPACE_NAME:-API Environment Fixture}"
ENV_FIXTURE_TEST_RECIPIENT_A="${ENV_FIXTURE_TEST_RECIPIENT_A:-api-env-${ENV_FIXTURE_SUFFIX}@test.example.com}"
ENV_FIXTURE_TEST_RECIPIENT_B="${ENV_FIXTURE_TEST_RECIPIENT_B:-api-env-review-${ENV_FIXTURE_SUFFIX}@test.example.com}"
ENV_FIXTURE_PROD_KEY_NAME="${ENV_FIXTURE_PROD_KEY_NAME:-API Contract Prod Environment Key}"
ENV_FIXTURE_TEST_KEY_NAME="${ENV_FIXTURE_TEST_KEY_NAME:-API Contract Test Environment Key}"
EXTERNAL_PROFILE_SLUG="${EXTERNAL_PROFILE_SLUG:-partner-portal}"
EXTERNAL_PROFILE_TOKEN="${EXTERNAL_PROFILE_TOKEN:-external-e2e-token}"
WORKSPACE_CODE="${WORKSPACE_CODE:-${SYSTEM_WORKSPACE_CODE:-main}}"

run_repo_tests_without_system_env() {
  local target="$1"
  env \
    -u SENDA_DATABASE_URL \
    -u SENDA_BASE_URL \
    -u MAILPIT_URL \
    -u KEYCLOAK_BASE_URL \
    -u FRONTEND_BASE_URL \
    -u AUTH_URL \
    -u AUTH_SECRET \
    -u AUTH_TRUST_HOST \
    -u AUTH_OIDC_ISSUER \
    -u AUTH_OIDC_ID \
    -u AUTH_OIDC_SECRET \
    make -C "$ROOT_DIR" "$target"
}

bootstrap_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"

  if [[ -n "$body" ]]; then
    curl -sS -w '\n%{http_code}' -X "$method" "$SENDA_BASE_URL$path" \
      -H "Authorization: Bearer $E2E_BOOTSTRAP_TOKEN" \
      -H 'Content-Type: application/json' \
      --data "$body"
  else
    curl -sS -w '\n%{http_code}' -X "$method" "$SENDA_BASE_URL$path" \
      -H "Authorization: Bearer $E2E_BOOTSTRAP_TOKEN"
  fi
}

bootstrap_api_expect() {
  local expected_status="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local response status payload

  response="$(bootstrap_api_request "$method" "$path" "$body")"
  status="$(printf '%s' "$response" | tail -n1)"
  payload="$(printf '%s' "$response" | sed '$d')"

  if [[ "$status" != "$expected_status" ]]; then
    echo "bootstrap ${method} failed: expected=${expected_status} actual=${status} path=${path} body=${payload}" >&2
    return 1
  fi

  printf '%s\n' "$payload"
}

bootstrap_api_status() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local response
  response="$(bootstrap_api_request "$method" "$path" "$body")"
  printf '%s' "$response" | tail -n1
}

bootstrap_api_get() {
  bootstrap_api_expect "200" GET "$1"
}

bootstrap_api_request_with_headers() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  shift 3 || true

  local curl_args=(
    -sS
    -w $'\n%{http_code}'
    -X "$method"
    "$SENDA_BASE_URL$path"
    -H "Authorization: Bearer $E2E_BOOTSTRAP_TOKEN"
  )

  local header
  for header in "$@"; do
    curl_args+=(-H "$header")
  done

  if [[ -n "$body" ]]; then
    curl_args+=(-H 'Content-Type: application/json' --data "$body")
  fi

  curl "${curl_args[@]}"
}

external_api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  shift 3 || true

  local curl_args=(
    -sS
    -w $'\n%{http_code}'
    -X "$method"
    "$SENDA_BASE_URL$path"
  )

  local header
  for header in "$@"; do
    curl_args+=(-H "$header")
  done

  if [[ -n "$body" ]]; then
    curl_args+=(-H 'Content-Type: application/json' --data "$body")
  fi

  curl "${curl_args[@]}"
}

ensure_tenant_workspace() {
  local workspace_code="$1"
  local workspace_name="$2"
  local status

  status="$(bootstrap_api_status POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces" "{\"code\":\"${workspace_code}\",\"name\":\"${workspace_name}\"}")"
  if [[ "$status" != "201" && "$status" != "409" ]]; then
    echo "failed to ensure workspace code=${workspace_code} status=${status}" >&2
    return 1
  fi
}

resolve_workspace_id() {
  local workspace_code="$1"
  bootstrap_api_get "/api/v1/manage/tenants/${TENANT_CODE}/workspaces" \
    | jq -r --arg code "$workspace_code" '.items[] | select(.code == $code) | .id' \
    | head -n1
}

resolve_system_adapter_id() {
  local adapter_name="$1"
  bootstrap_api_get "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters" \
    | jq -r --arg name "$adapter_name" '.items[] | select(.name == $name) | .id' \
    | head -n1
}

ensure_system_gmail_adapter() {
  local adapter_id
  adapter_id="$(resolve_system_adapter_id "$SHARED_GMAIL_NAME")"
  if [[ -n "$adapter_id" ]]; then
    printf '%s\n' "$adapter_id"
    return 0
  fi

  adapter_id="$(
    bootstrap_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters" "{
      \"name\": \"${SHARED_GMAIL_NAME}\",
      \"adapter_type\": \"gmail\",
      \"config\": {
        \"service_account_json\": \"{\\\"type\\\":\\\"service_account\\\",\\\"project_id\\\":\\\"system-test\\\"}\",
        \"delegate_email\": \"${SHARED_GMAIL_DELEGATE_EMAIL}\"
      },
      \"is_default\": false,
      \"rate_limit_per_second\": 2
    }" | jq -r '.id // empty'
  )"

  if [[ -z "$adapter_id" ]]; then
    echo "failed to create shared gmail adapter fixture" >&2
    return 1
  fi

  printf '%s\n' "$adapter_id"
}

ensure_system_ses_adapter() {
  local adapter_id
  adapter_id="$(resolve_system_adapter_id "$SHARED_SES_NAME")"
  if [[ -n "$adapter_id" ]]; then
    printf '%s\n' "$adapter_id"
    return 0
  fi

  adapter_id="$(
    bootstrap_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters" "{
      \"name\": \"${SHARED_SES_NAME}\",
      \"adapter_type\": \"ses\",
      \"config\": {
        \"region\": \"us-east-1\",
        \"access_key_id\": \"test\",
        \"secret_access_key\": \"test\",
        \"endpoint_url\": \"${AWS_SIM_INTERNAL_URL}\"
      },
      \"is_default\": false,
      \"rate_limit_per_second\": 100
    }" | jq -r '.id // empty'
  )"

  if [[ -z "$adapter_id" ]]; then
    echo "failed to create shared ses adapter fixture" >&2
    return 1
  fi

  printf '%s\n' "$adapter_id"
}

sync_adapter_identities() {
  local adapter_id="$1"
  bootstrap_api_expect "200" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters/${adapter_id}/identities/sync" >/dev/null
}

create_aws_sim_identity() {
  local identity="$1"
  systemtest aws-sim-create-identity \
    --endpoint "$AWS_SIM_BASE_URL" \
    --identity "$identity" >/dev/null
}

resolve_identity_id() {
  local adapter_id="$1"
  local identity="$2"
  bootstrap_api_get "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters/${adapter_id}/identities" \
    | jq -r --arg identity "$identity" '.[] | select(.identity == $identity) | .id' \
    | head -n1
}

ensure_manual_identity() {
  local adapter_id="$1"
  local identity="$2"
  local display_name="${3:-}"
  local identity_id

  identity_id="$(resolve_identity_id "$adapter_id" "$identity")"
  if [[ -n "$identity_id" ]]; then
    printf '%s\n' "$identity_id"
    return 0
  fi

  if [[ -n "$display_name" ]]; then
    identity_id="$(
      bootstrap_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters/${adapter_id}/identities" \
        "{\"identity\":\"${identity}\",\"display_name\":\"${display_name}\"}" \
        | jq -r '.id // empty'
    )"
  else
    identity_id="$(
      bootstrap_api_expect "201" POST "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters/${adapter_id}/identities" \
        "{\"identity\":\"${identity}\"}" \
        | jq -r '.id // empty'
    )"
  fi

  if [[ -z "$identity_id" ]]; then
    echo "failed to create manual identity fixture identity=${identity}" >&2
    return 1
  fi

  printf '%s\n' "$identity_id"
}

delete_manual_identity_if_exists() {
  local adapter_id="$1"
  local identity="$2"
  local identity_id status

  identity_id="$(resolve_identity_id "$adapter_id" "$identity")"
  if [[ -z "$identity_id" ]]; then
    return 0
  fi

  status="$(bootstrap_api_status DELETE "/api/v1/manage/tenants/${TENANT_CODE}/workspaces/_system/adapters/${adapter_id}/identities/${identity_id}")"
  if [[ "$status" != "204" && "$status" != "404" ]]; then
    echo "failed to delete manual identity fixture identity=${identity} status=${status}" >&2
    return 1
  fi
}

ensure_external_integration_profile() {
  bootstrap_api_expect "200" PUT "/api/v1/manage/config" "$(cat <<EOF_JSON
{
  "external_integrations": {
    "profiles": [
      {
        "slug": "${EXTERNAL_PROFILE_SLUG}",
        "name": "Partner Portal",
        "description": "System test external integration profile",
        "enabled": true,
        "auth_method_name": "e2e-signed-token",
        "resolver_name": "e2e-workspace-resolver",
        "allowed_origins": ["http://localhost:3000"],
        "allowed_headers": ["x-tenant-code", "x-senda-external-token"],
        "required_headers": ["x-tenant-code"],
        "capabilities": {
          "list_templates": true,
          "view_versions": true,
          "edit_versions": true,
          "publish_versions": true,
          "test_send": true,
          "builder_access": true,
          "metadata_access": true,
          "locale_access": true
        }
      }
    ]
  }
}
EOF_JSON
)" >/dev/null
}

run_environment_contract_checks() {
  log "api-contract-tester: running environment contract checks"

  ensure_tenant_workspace "$ENV_FIXTURE_WORKSPACE_CODE" "$ENV_FIXTURE_WORKSPACE_NAME"
  ensure_external_integration_profile

  local prod_list test_list prod_workspace prod_reject_response prod_reject_status prod_reject_payload
  local test_update_response test_key_response prod_key_response test_key prod_key test_key_id prod_key_id
  local reset_prod_response reset_prod_status reset_prod_payload reset_test_status
  local external_missing_response external_missing_status external_missing_payload
  local external_invalid_response external_invalid_status external_invalid_payload
  local external_ok_response external_ok_status bootstrap_test_status
  local env_prod_base="/api/v1/manage/environments/prod/tenants/${TENANT_CODE}/workspaces/${ENV_FIXTURE_WORKSPACE_CODE}"
  local env_test_base="/api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces/${ENV_FIXTURE_WORKSPACE_CODE}"
  local external_base="/api/v1/external/${EXTERNAL_PROFILE_SLUG}/tenants/${TENANT_CODE}/workspaces/${ENV_FIXTURE_WORKSPACE_CODE}"

  prod_list="$(bootstrap_api_get "/api/v1/manage/tenants/${TENANT_CODE}/workspaces")"
  printf '%s\n' "$prod_list" | jq -e --arg code "$ENV_FIXTURE_WORKSPACE_CODE" '
    any(.items[]; .code == $code and .environment == "prod")
  ' >/dev/null

  test_list="$(bootstrap_api_get "/api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces")"
  printf '%s\n' "$test_list" | jq -e --arg code "$ENV_FIXTURE_WORKSPACE_CODE" '
    any(.items[]; .code == $code and .environment == "test")
  ' >/dev/null

  prod_workspace="$(bootstrap_api_get "$env_prod_base")"
  printf '%s\n' "$prod_workspace" | jq -e '
    .environment == "prod"
    and ((.test_recipient_addresses // []) | length == 0)
    and ((.test_recipient_mode // "replace") == "replace")
  ' >/dev/null

  prod_reject_response="$(bootstrap_api_request_with_headers PUT "$env_prod_base" "$(cat <<EOF_JSON
{"test_recipient_mode":"append","test_recipient_addresses":["${ENV_FIXTURE_TEST_RECIPIENT_A}","${ENV_FIXTURE_TEST_RECIPIENT_B}"]}
EOF_JSON
)")"
  prod_reject_status="$(printf '%s\n' "$prod_reject_response" | tail -n1)"
  prod_reject_payload="$(printf '%s\n' "$prod_reject_response" | sed '$d')"
  [[ "$prod_reject_status" == "422" ]]
  printf '%s\n' "$prod_reject_payload" | jq -e '
    .error.message == "validation failed"
    and ((.error.fields // .error.details // []) | any(.field == "test_recipient_mode"))
  ' >/dev/null

  test_update_response="$(bootstrap_api_expect "200" PUT "$env_test_base" "$(cat <<EOF_JSON
{"test_recipient_mode":"append","test_recipient_addresses":["${ENV_FIXTURE_TEST_RECIPIENT_A}","${ENV_FIXTURE_TEST_RECIPIENT_B}"]}
EOF_JSON
)")"
  printf '%s\n' "$test_update_response" | jq -e \
    --arg addr1 "$ENV_FIXTURE_TEST_RECIPIENT_A" \
    --arg addr2 "$ENV_FIXTURE_TEST_RECIPIENT_B" '
      .environment == "test"
      and .test_recipient_mode == "append"
      and (.test_recipient_addresses | index($addr1))
      and (.test_recipient_addresses | index($addr2))
    ' >/dev/null

  prod_workspace="$(bootstrap_api_get "$env_prod_base")"
  printf '%s\n' "$prod_workspace" | jq -e '
    .environment == "prod"
    and ((.test_recipient_addresses // []) | length == 0)
    and ((.test_recipient_mode // "replace") == "replace")
  ' >/dev/null

  reset_prod_response="$(bootstrap_api_request_with_headers POST "${env_prod_base}/runtime/reset" "")"
  reset_prod_status="$(printf '%s\n' "$reset_prod_response" | tail -n1)"
  reset_prod_payload="$(printf '%s\n' "$reset_prod_response" | sed '$d')"
  [[ "$reset_prod_status" == "409" ]]
  printf '%s\n' "$reset_prod_payload" | jq -e '
    .error.code == "TEST_ENVIRONMENT_REQUIRED"
  ' >/dev/null

  reset_test_status="$(bootstrap_api_status POST "${env_test_base}/runtime/reset")"
  [[ "$reset_test_status" == "204" ]]

  prod_key_response="$(bootstrap_api_expect "201" POST "${env_prod_base}/api-keys" "$(cat <<EOF_JSON
{"name":"${ENV_FIXTURE_PROD_KEY_NAME}"}
EOF_JSON
)")"
  prod_key="$(printf '%s\n' "$prod_key_response" | jq -r '.key')"
  prod_key_id="$(printf '%s\n' "$prod_key_response" | jq -r '.id')"
  [[ "$prod_key" == senda_prod_* ]]

  test_key_response="$(bootstrap_api_expect "201" POST "${env_test_base}/api-keys" "$(cat <<EOF_JSON
{"name":"${ENV_FIXTURE_TEST_KEY_NAME}"}
EOF_JSON
)")"
  test_key="$(printf '%s\n' "$test_key_response" | jq -r '.key')"
  test_key_id="$(printf '%s\n' "$test_key_response" | jq -r '.id')"
  [[ "$test_key" == senda_test_* ]]

  bootstrap_api_expect "204" DELETE "${env_prod_base}/api-keys/${prod_key_id}" >/dev/null
  bootstrap_api_expect "204" DELETE "${env_test_base}/api-keys/${test_key_id}" >/dev/null

  bootstrap_test_status="$(external_api_request GET "/api/v1/external/${EXTERNAL_PROFILE_SLUG}/environments/test/bootstrap" "" | tail -n1)"
  [[ "$bootstrap_test_status" == "200" ]]

  external_missing_response="$(external_api_request GET "${external_base}/template-types?token=${EXTERNAL_PROFILE_TOKEN}" "" "X-Tenant-Code: ${TENANT_CODE}")"
  external_missing_status="$(printf '%s\n' "$external_missing_response" | tail -n1)"
  external_missing_payload="$(printf '%s\n' "$external_missing_response" | sed '$d')"
  [[ "$external_missing_status" == "400" ]]
  printf '%s\n' "$external_missing_payload" | jq -e '
    .error.message == "missing required header X-Senda-Environment"
  ' >/dev/null

  external_invalid_response="$(external_api_request GET "${external_base}/template-types?token=${EXTERNAL_PROFILE_TOKEN}" "" "X-Tenant-Code: ${TENANT_CODE}" "X-Senda-Environment: sandbox")"
  external_invalid_status="$(printf '%s\n' "$external_invalid_response" | tail -n1)"
  external_invalid_payload="$(printf '%s\n' "$external_invalid_response" | sed '$d')"
  [[ "$external_invalid_status" == "400" ]]
  printf '%s\n' "$external_invalid_payload" | jq -e '
    .error.message == "invalid X-Senda-Environment header"
  ' >/dev/null

  external_ok_response="$(external_api_request GET "${external_base}/template-types?token=${EXTERNAL_PROFILE_TOKEN}" "" "X-Tenant-Code: ${TENANT_CODE}" "X-Senda-Environment: test")"
  external_ok_status="$(printf '%s\n' "$external_ok_response" | tail -n1)"
  [[ "$external_ok_status" == "200" ]]

  ENVIRONMENT_CONTRACT_GATES=$(cat <<EOF_GATES
4. Environment-scoped management contract:
   - \`GET /api/v1/manage/tenants/${TENANT_CODE}/workspaces\` resolves prod workspaces by default.
   - \`GET /api/v1/manage/environments/test/tenants/${TENANT_CODE}/workspaces\` resolves the test workspace set.
   - \`GET|PUT /api/v1/manage/environments/{prod|test}/tenants/${TENANT_CODE}/workspaces/${ENV_FIXTURE_WORKSPACE_CODE}\` keeps test-recipient state isolated to the test environment.
   - \`POST /runtime/reset\` rejects prod with \`TEST_ENVIRONMENT_REQUIRED\` and succeeds for test.
5. Environment-scoped API key generation returns \`senda_prod_\` for prod and \`senda_test_\` for test.
6. External integration contract:
   - \`GET /api/v1/external/${EXTERNAL_PROFILE_SLUG}/environments/test/bootstrap\` succeeds.
   - Scoped external routes reject missing/invalid \`X-Senda-Environment\`.
   - Scoped external routes accept \`X-Senda-Environment: test\` when the rest of the request is valid.
EOF_GATES
)
}

log "api-contract-tester: seeding keycloak users + RBAC"
seed_keycloak_users
seed_rbac_memberships

log "api-contract-tester: ensuring deterministic E2E tenant exists (test-corp)"
E2E_BOOTSTRAP_TOKEN="$(systemtest token --email "$SUPERADMIN_EMAIL" --secret "$SENDA_E2E_JWT_SECRET" | tail -n1)"
E2E_TENANT_BODY="$ARTIFACT_DIR/e2e-tenant-create.json"
E2E_TENANT_STATUS="$(curl -sS -o "$E2E_TENANT_BODY" -w '%{http_code}' \
  -X POST "$SENDA_BASE_URL/api/v1/manage/tenants" \
  -H "Authorization: Bearer $E2E_BOOTSTRAP_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"code":"test-corp","name":"Test Corp"}')"
if [[ "$E2E_TENANT_STATUS" != "201" && "$E2E_TENANT_STATUS" != "409" ]]; then
  log "api-contract-tester: failed to ensure test-corp tenant (status=$E2E_TENANT_STATUS)"
  cat "$E2E_TENANT_BODY"
  exit 1
fi

log "api-contract-tester: seeding deterministic E2E fixture RBAC (test-corp/main)"
systemtest seed-rbac \
  --base-url "$SENDA_BASE_URL" \
  --email "$SUPERADMIN_EMAIL" \
  --secret "$SENDA_E2E_JWT_SECRET" \
  --tenant-code "test-corp" \
  --tenant-name "Test Corp" \
  --workspace-code "main" \
  --workspace-name "Main Workspace" \
  --superadmin-email "$SUPERADMIN_EMAIL" \
  --tenant-admin-email "$TENANT_ADMIN_EMAIL" \
  --workspace-admin-email "$WORKSPACE_ADMIN_EMAIL" \
  --workspace-editor-email "$WORKSPACE_EDITOR_EMAIL" \
  --workspace-viewer-email "$WORKSPACE_VIEWER_EMAIL" \
  --no-member-email "$NO_MEMBER_EMAIL"

AWS_SIM_BASE_URL="$(jq -r '.services.aws_sim // empty' "$ENV_REPORT_FILE")"
if [[ -z "$AWS_SIM_BASE_URL" ]]; then
  log "api-contract-tester: env-report missing aws_sim service URL"
  exit 1
fi

log "api-contract-tester: ensuring deterministic shared-adapter Postman fixtures"
ensure_tenant_workspace "$SHARED_WORKSPACE_A_CODE" "$SHARED_WORKSPACE_A_NAME"
ensure_tenant_workspace "$SHARED_WORKSPACE_B_CODE" "$SHARED_WORKSPACE_B_NAME"

SHARED_WORKSPACE_A_ID="$(resolve_workspace_id "$SHARED_WORKSPACE_A_CODE")"
SHARED_WORKSPACE_B_ID="$(resolve_workspace_id "$SHARED_WORKSPACE_B_CODE")"
if [[ -z "$SHARED_WORKSPACE_A_ID" || -z "$SHARED_WORKSPACE_B_ID" ]]; then
  echo "failed to resolve deterministic shared workspace ids" >&2
  exit 1
fi

SHARED_GMAIL_ADAPTER_ID="$(ensure_system_gmail_adapter)"
SHARED_SES_ADAPTER_ID="$(ensure_system_ses_adapter)"

create_aws_sim_identity "$SHARED_SES_DOMAIN"
sync_adapter_identities "$SHARED_SES_ADAPTER_ID"

SHARED_SES_IDENTITY_ID="$(ensure_manual_identity "$SHARED_SES_ADAPTER_ID" "$SHARED_SES_DEFAULT_EMAIL" "API Contract Shared Sender")"
delete_manual_identity_if_exists "$SHARED_SES_ADAPTER_ID" "$SHARED_SES_TEMP_EMAIL"

if [[ "${SYSTEM_API_CONTRACT_FULL:-0}" == "1" ]]; then
  log "api-contract-tester: make test"
  run_repo_tests_without_system_env test

  log "api-contract-tester: make test-integration"
  run_repo_tests_without_system_env test-integration
else
  log "api-contract-tester: skipping make test + make test-integration (covered by dedicated backend gates; opt in with SYSTEM_API_CONTRACT_FULL=1)"
fi

POSTGRES_CONTAINER="$(jq -r '.runtime.containers.postgres // empty' "$ENV_REPORT_FILE")"
if [[ -z "$POSTGRES_CONTAINER" ]]; then
  log "api-contract-tester: env-report missing postgres container name"
  exit 1
fi

POSTGRES_PORT="$(docker port "$POSTGRES_CONTAINER" 5432/tcp | awk -F: 'NR==1 {print $NF}')"
if [[ -z "$POSTGRES_PORT" ]]; then
  log "api-contract-tester: failed to resolve postgres mapped port for $POSTGRES_CONTAINER"
  exit 1
fi

export MAILPIT_URL="${MAILPIT_BASE_URL}"
export SENDA_DATABASE_URL="postgres://senda:senda@127.0.0.1:${POSTGRES_PORT}/senda?sslmode=disable"

if [[ "${SYSTEM_API_CONTRACT_E2E:-0}" == "1" ]]; then
  log "api-contract-tester: make test-e2e-run (explicit opt-in)"
  make -C "$ROOT_DIR" test-e2e-run
else
  log "api-contract-tester: skipping make test-e2e-run (covered by dedicated nightly security/chaos suites or explicit local runs)"
fi

ensure_runtime_env

set -a
# shellcheck disable=SC1090
source "$RUNTIME_ENV_FILE"
set +a

run_environment_contract_checks

log "api-contract-tester: executing Postman collection with newman"
set +e
corepack pnpm dlx newman run "$COLLECTION" \
  -e "$ENV_FILE" \
  --env-var "base_url=$SENDA_BASE_URL" \
  --env-var "oidc_token=$OIDC_TOKEN" \
  --env-var "api_key=$API_KEY" \
  --env-var "tenant_code=$TENANT_CODE" \
  --env-var "workspace_code=$WORKSPACE_CODE" \
  --env-var "shared_gmail_adapter_id=$SHARED_GMAIL_ADAPTER_ID" \
  --env-var "shared_ses_adapter_id=$SHARED_SES_ADAPTER_ID" \
  --env-var "shared_ses_identity_id=$SHARED_SES_IDENTITY_ID" \
  --env-var "shared_ses_domain=$SHARED_SES_DOMAIN" \
  --env-var "shared_ses_temp_email=$SHARED_SES_TEMP_EMAIL" \
  --env-var "shared_workspace_a_id=$SHARED_WORKSPACE_A_ID" \
  --env-var "shared_workspace_b_id=$SHARED_WORKSPACE_B_ID" \
  --reporters "cli,json,junit" \
  --reporter-json-export "$JSON_REPORT" \
  --reporter-junit-export "$JUNIT_REPORT"
NEWMAN_EXIT=$?
set -e

NEWMAN_SUMMARY="$(node - <<'NODE' "$JSON_REPORT" "$NEWMAN_EXIT"
const fs=require('fs');
const p=process.argv[2];
const exitCode=Number(process.argv[3]||0);
let run={}, s={}, failures=0;
if (fs.existsSync(p)) {
  const data=JSON.parse(fs.readFileSync(p,'utf8'));
  run=data.run||{};
  s=run.stats||{};
  failures=(run.failures||[]).length;
}
const out={
  requests:(s.requests||{}).total||0,
  assertions:(s.assertions||{}).total||0,
  failures,
  newman_exit: exitCode
};
process.stdout.write(JSON.stringify(out));
NODE
)"

if [[ "${SYSTEM_API_CONTRACT_FULL:-0}" == "1" ]]; then
  EXECUTED_GATES=$(cat <<EOF_GATES
1. \`make test\` (unit tests).
2. \`make test-integration\` (integration tests).
3. Newman run over collection: \`$COLLECTION\`.
${ENVIRONMENT_CONTRACT_GATES}
7. Optional deterministic E2E backend suites are available via \`SYSTEM_API_CONTRACT_E2E=1\`, but are not part of the default API-contract gate because nightly already runs dedicated security/chaos coverage separately.
EOF_GATES
)
else
  EXECUTED_GATES=$(cat <<EOF_GATES
1. Seed deterministic auth + RBAC fixtures for the system test tenant/workspace.
2. Newman run over collection: \`$COLLECTION\`.
${ENVIRONMENT_CONTRACT_GATES}
7. Dedicated backend/unit/integration/E2E suites are expected to run in their own gates; opt in here with \`SYSTEM_API_CONTRACT_FULL=1\` and/or \`SYSTEM_API_CONTRACT_E2E=1\` when explicitly needed.
EOF_GATES
)
fi

cat >"$REPORT_PATH" <<EOF_MD
# API Contract Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Base URL: $SENDA_BASE_URL
- Runtime env tenant_code: $TENANT_CODE
- Runtime env workspace_code: $WORKSPACE_CODE
- Newman summary: $NEWMAN_SUMMARY

## Executed Gates

$EXECUTED_GATES

## Artifacts

- JSON report: \`$JSON_REPORT\`
- JUnit report: \`$JUNIT_REPORT\`
- Runtime env: \`$RUNTIME_ENV_FILE\`
EOF_MD

log "api-contract-tester: report written -> $REPORT_PATH"

if [[ "$NEWMAN_EXIT" -ne 0 ]]; then
  log "api-contract-tester: newman failed with exit=$NEWMAN_EXIT"
  exit "$NEWMAN_EXIT"
fi
