#!/usr/bin/env bash
set -euo pipefail

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
  go run "$ROOT_DIR/cmd/systemtest" aws-sim-create-identity \
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

log "api-contract-tester: seeding keycloak users + RBAC"
seed_keycloak_users
seed_rbac_memberships

log "api-contract-tester: ensuring deterministic E2E tenant exists (test-corp)"
E2E_BOOTSTRAP_TOKEN="$(go run "$ROOT_DIR/cmd/systemtest" token --email "$SUPERADMIN_EMAIL" --secret "$SENDA_E2E_JWT_SECRET" | tail -n1)"
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
go run "$ROOT_DIR/cmd/systemtest" seed-rbac \
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

if [[ "$SYSTEM_MODE" == "nightly" || "${SYSTEM_API_CONTRACT_FULL:-0}" == "1" ]]; then
  log "api-contract-tester: make test"
  make -C "$ROOT_DIR" test

  log "api-contract-tester: make test-integration"
  make -C "$ROOT_DIR" test-integration
else
  log "api-contract-tester: skipping make test + make test-integration in PR mode (covered by dedicated gates)"
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

if [[ "$SYSTEM_MODE" == "nightly" || "${SYSTEM_API_CONTRACT_FULL:-0}" == "1" ]]; then
  log "api-contract-tester: make test-e2e-run"
  make -C "$ROOT_DIR" test-e2e-run
else
  log "api-contract-tester: skipping make test-e2e-run in PR mode (run locally before PR when change scope requires it)"
fi

ensure_runtime_env

set -a
# shellcheck disable=SC1090
source "$RUNTIME_ENV_FILE"
set +a

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

if [[ "$SYSTEM_MODE" == "nightly" || "${SYSTEM_API_CONTRACT_FULL:-0}" == "1" ]]; then
  EXECUTED_GATES=$(cat <<EOF_GATES
1. \`make test\` (unit tests).
2. \`make test-integration\` (integration tests).
3. \`make test-e2e-run\` (deterministic E2E backend suites).
4. Newman run over collection: \`$COLLECTION\`.
EOF_GATES
)
else
  EXECUTED_GATES=$(cat <<EOF_GATES
1. Seed deterministic auth + RBAC fixtures for the system test tenant/workspace.
2. Newman run over collection: \`$COLLECTION\`.
3. Dedicated backend/unit/integration/E2E suites are expected to run outside this PR smoke gate.
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
