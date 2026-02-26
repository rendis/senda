#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd go
require_cmd make
require_cmd npm
require_cmd node

REPORT_PATH="$ARTIFACT_DIR/api-contract-report.md"
RUNTIME_ENV_FILE="${RUNTIME_ENV_FILE:-$ARTIFACT_DIR/runtime.env}"
COLLECTION="$ROOT_DIR/docs/postman/senda-api-v1.postman_collection.json"
ENV_FILE="$ROOT_DIR/docs/postman/senda-local.postman_environment.json"
JSON_REPORT="$ARTIFACT_DIR/newman-report.json"
JUNIT_REPORT="$ARTIFACT_DIR/newman-junit.xml"

if ! command -v newman >/dev/null 2>&1; then
  log "installing newman"
  npm install -g newman >/dev/null
fi

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

log "api-contract-tester: make test"
make -C "$ROOT_DIR" test

log "api-contract-tester: make test-integration"
make -C "$ROOT_DIR" test-integration

log "api-contract-tester: make test-e2e-run"
make -C "$ROOT_DIR" test-e2e-run

ensure_runtime_env

set -a
# shellcheck disable=SC1090
source "$RUNTIME_ENV_FILE"
set +a

log "api-contract-tester: executing Postman collection with newman"
set +e
newman run "$COLLECTION" \
  -e "$ENV_FILE" \
  --env-var "base_url=$SENDA_BASE_URL" \
  --env-var "oidc_token=$OIDC_TOKEN" \
  --env-var "api_key=$API_KEY" \
  --env-var "tenant_code=$TENANT_CODE" \
  --env-var "workspace_code=$WORKSPACE_CODE" \
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

cat >"$REPORT_PATH" <<EOF_MD
# API Contract Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Base URL: $SENDA_BASE_URL
- Runtime env tenant_code: $TENANT_CODE
- Runtime env workspace_code: $WORKSPACE_CODE
- Newman summary: $NEWMAN_SUMMARY

## Executed Gates

1. \`make test\` (unit tests).
2. \`make test-integration\` (integration tests).
3. \`make test-e2e-run\` (deterministic E2E backend suites).
4. Newman run over collection: \`$COLLECTION\`.

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
