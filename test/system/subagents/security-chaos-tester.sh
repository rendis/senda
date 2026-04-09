#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd make
require_cmd jq

load_env_report "$ENV_REPORT_FILE"

REPORT_PATH="$ARTIFACT_DIR/security-chaos-report.md"

log "security-chaos-tester: running deterministic security suite (TestS*)"
(
  cd "$ROOT_DIR"
  SENDA_BASE_URL="$SENDA_BASE_URL" \
  MAILPIT_URL="$MAILPIT_BASE_URL" \
  SENDA_E2E_JWT_SECRET="$SENDA_E2E_JWT_SECRET" \
  go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run '^TestS'
)

log "security-chaos-tester: running chaos suite (TestC*)"
make -C "$ROOT_DIR" test-e2e-chaos-run

cat >"$REPORT_PATH" <<EOF_MD
# Security + Chaos Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Base URL: $SENDA_BASE_URL

## Executed Suites

1. Security suite (\`TestS*\`) with E2E tag.
2. Chaos suite (\`TestC*\`) using \`make test-e2e-chaos-run\`.

## Notes

- Any non-zero exit in these suites is release blocking under the system test policy.
EOF_MD

log "security-chaos-tester: report written -> $REPORT_PATH"
