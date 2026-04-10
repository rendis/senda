#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

ACTION="${1:-up}"
ENV_REPORT_PATH="${2:-$ARTIFACT_DIR/env-report.json}"

require_cmd go
require_cmd curl
require_cmd jq

stack_cmd() {
  systemtest stack "$@"
}

health_check() {
  curl -fsS "$SENDA_BASE_URL/health" >/dev/null
  curl -fsS "$MAILPIT_BASE_URL/api/v1/messages" >/dev/null
  curl -fsS "$KEYCLOAK_BASE_URL/realms/${KEYCLOAK_REALM}/.well-known/openid-configuration" >/dev/null
}

case "$ACTION" in
  up)
    log "infra-orchestrator: stack up"
    stack_cmd up --mode "$SYSTEM_MODE" --out "$ENV_REPORT_PATH"
    load_env_report "$ENV_REPORT_PATH"
    log "infra-orchestrator: health checks"
    health_check
    log "infra-orchestrator: environment ready"
    ;;
  down)
    log "infra-orchestrator: stack down"
    stack_cmd down --out "$ENV_REPORT_PATH"
    ;;
  status)
    health_check
    log "infra-orchestrator: healthy"
    ;;
  *)
    echo "usage: $0 <up|down|status> [env-report-path]" >&2
    exit 2
    ;;
esac
