#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd agent-browser
require_cmd timeout
require_cmd jq
require_cmd curl

REPORT_PATH="$ARTIFACT_DIR/ui-onboarding-auth-guard-report.md"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-onboarding-auth-guard"
SESSION_NAME="senda-onboarding-auth-guard-$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"

mkdir -p "$SCREENSHOT_DIR"

ab() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@"
}

ab_json() {
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$SESSION_NAME" "$@" --json
}

cleanup() {
  timeout 5s agent-browser --session "$SESSION_NAME" close >/dev/null 2>&1 || true
}

trap cleanup EXIT

load_env_report "$ENV_REPORT_FILE"

if ! curl -fsS "$FRONTEND_BASE_URL/login" >/dev/null 2>&1; then
  start_frontend
fi

log "ui-onboarding-auth-guard: seeding stale onboarding session state"
ab open "$FRONTEND_BASE_URL/login" >/dev/null
ab wait 1000 >/dev/null
ab eval "(() => {
  window.sessionStorage.setItem('senda-onboarding', JSON.stringify({ step: 2, tenantCode: 'stale-tenant' }));
  return 'ok';
})()" >/dev/null

log "ui-onboarding-auth-guard: opening onboarding while unauthenticated"
ab open "$FRONTEND_BASE_URL/onboarding" >/dev/null
ab wait 1500 >/dev/null

FINAL_URL="$(ab_json get url | jq -r '.data.url // ""')"
BODY_TEXT="$(ab_json eval '(() => (document.body?.innerText || "").replace(/\s+/g, " ").trim())()' | jq -r '.data.result // ""')"

ab screenshot "$SCREENSHOT_DIR/final-state.png" >/dev/null || true

PASS=1
NOTE=""

if [[ "$FINAL_URL" != *"/login"* ]]; then
  PASS=0
  NOTE="expected redirect to /login, got $FINAL_URL"
fi

if [[ "$BODY_TEXT" == *"Create your organization"* || "$BODY_TEXT" == *"Create your workspace"* ]]; then
  PASS=0
  if [[ -n "$NOTE" ]]; then
    NOTE="$NOTE; leaked onboarding step content"
  else
    NOTE="leaked onboarding step content"
  fi
fi

cat >"$REPORT_PATH" <<EOF_MD
# UI Onboarding Auth Guard Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Frontend URL: $FRONTEND_BASE_URL
- Final URL: $FINAL_URL
- Result: $(if [[ "$PASS" -eq 1 ]]; then echo "pass"; else echo "fail"; fi)
- Note: ${NOTE:-none}
- Screenshot: \`$SCREENSHOT_DIR/final-state.png\`

## Body Text Snapshot

\`\`\`
$BODY_TEXT
\`\`\`
EOF_MD

log "ui-onboarding-auth-guard: report written -> $REPORT_PATH"

if [[ "$PASS" -ne 1 ]]; then
  echo "$NOTE" >&2
  exit 1
fi
