#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

ACTION="${1:-up}"
ENV_REPORT_PATH="${2:-$ARTIFACT_DIR/env-report.json}"
COMPOSE_FILE="$ROOT_DIR/docker/docker-compose.e2e.yml"

require_cmd docker
require_cmd curl

health_check() {
  curl -fsS "$SENDA_BASE_URL/health" >/dev/null
  curl -fsS "$MAILPIT_BASE_URL/api/v1/messages" >/dev/null
  curl -fsS "$KEYCLOAK_BASE_URL/realms/${KEYCLOAK_REALM}/.well-known/openid-configuration" >/dev/null
}

write_env_report() {
  local timestamp
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  cat >"$ENV_REPORT_PATH" <<JSON
{
  "timestamp": "$timestamp",
  "status": "healthy",
  "services": {
    "senda": "$SENDA_BASE_URL",
    "mailpit": "$MAILPIT_BASE_URL",
    "keycloak": "$KEYCLOAK_BASE_URL"
  },
  "compose_file": "$COMPOSE_FILE"
}
JSON
}

case "$ACTION" in
  up)
    log "infra-orchestrator: docker compose up"
    docker compose -f "$COMPOSE_FILE" up -d --build --wait
    log "infra-orchestrator: health checks"
    health_check
    mkdir -p "$(dirname "$ENV_REPORT_PATH")"
    write_env_report
    log "infra-orchestrator: environment ready"
    ;;
  down)
    log "infra-orchestrator: docker compose down"
    docker compose -f "$COMPOSE_FILE" down -v
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
