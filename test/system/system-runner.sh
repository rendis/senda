#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-pr}"
SYSTEM_UI_VISUAL="${SYSTEM_UI_VISUAL:-0}"
SYSTEM_UI_FLOW="${SYSTEM_UI_FLOW:-}"
SYSTEM_SECURITY_CHAOS="${SYSTEM_SECURITY_CHAOS:-0}"

case "$MODE" in
  pr|nightly)
    ;;
  *)
    echo "usage: $0 <pr|nightly>" >&2
    exit 2
    ;;
esac

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/artifacts/system/$TIMESTAMP}"
STAGES_DIR="$ARTIFACT_DIR/stages"
STAGE_RESULTS="$ARTIFACT_DIR/stage-results.tsv"
ENV_REPORT="$ARTIFACT_DIR/env-report.json"

mkdir -p "$STAGES_DIR"
export SYSTEM_MODE="$MODE"
export ARTIFACT_DIR

source "$ROOT_DIR/test/system/subagents/lib.sh"

require_cmd go
require_cmd awk
require_cmd jq

echo -n >"$STAGE_RESULTS"
any_failed=0

log_stage() {
  local name="$1"
  local status="$2"
  local duration_ms="$3"
  local log_path="$4"
  echo -e "${name}\t${status}\t${duration_ms}\t${log_path}" >>"$STAGE_RESULTS"
}

run_stage() {
  local name="$1"
  shift
  local log_path="$STAGES_DIR/${name}.log"
  local start_ms end_ms duration_ms
  start_ms=$(( $(date +%s) * 1000 ))
  log "system-runner: stage=${name} start"
  set +e
  "$@" >"$log_path" 2>&1
  local code=$?
  set -e
  end_ms=$(( $(date +%s) * 1000 ))
  duration_ms=$(( end_ms - start_ms ))

  if [[ "$code" -eq 0 ]]; then
    log "system-runner: stage=${name} pass (${duration_ms}ms)"
    log_stage "$name" "pass" "$duration_ms" "$log_path"
  else
    log "system-runner: stage=${name} fail (${duration_ms}ms)"
    log_stage "$name" "fail" "$duration_ms" "$log_path"
    any_failed=1
  fi
}

skip_stage() {
  local name="$1"
  local reason="$2"
  local log_path="$STAGES_DIR/${name}.log"
  echo "$reason" >"$log_path"
  log "system-runner: stage=${name} skip (${reason})"
  log_stage "$name" "skip" 0 "$log_path"
}

run_visual_stage() {
  if [[ "$SYSTEM_UI_VISUAL" == "1" ]]; then
    run_stage "ui-visual" "$ROOT_DIR/test/system/subagents/ui-visual-tester.sh"
    return
  fi

  skip_stage \
    "ui-visual" \
    "disabled-by-default (set SYSTEM_UI_VISUAL=1 to enable baseline screenshots + diff)"
}

run_ui_flow_stage() {
  if [[ "$SYSTEM_UI_FLOW" != "1" ]]; then
    skip_stage \
      "ui-flow" \
      "disabled-by-default (set SYSTEM_UI_FLOW=1 to enable browser-based login/navigation coverage)"
    return
  fi

  run_stage "ui-flow" "$ROOT_DIR/test/system/subagents/ui-flow-tester.sh"
}

run_ui_template_type_slug_edit_stage() {
  run_stage "ui-template-type-slug-edit" "$ROOT_DIR/test/system/subagents/ui-template-type-slug-edit-tester.sh"
}

run_ui_workspace_management_stage() {
  run_stage "ui-workspace-management" "$ROOT_DIR/test/system/subagents/ui-workspace-management-tester.sh"
}

run_ui_injector_flow_stage() {
  run_stage "ui-injector-flow" "$ROOT_DIR/test/system/subagents/ui-injector-flow-tester.sh"
}

run_ui_onboarding_auth_guard_stage() {
  run_stage "ui-onboarding-auth-guard" "$ROOT_DIR/test/system/subagents/ui-onboarding-auth-guard-tester.sh"
}

run_ui_adapter_sharing_stage() {
  run_stage "ui-adapter-sharing" "$ROOT_DIR/test/system/subagents/ui-adapter-sharing-tester.sh"
}

run_security_chaos_stage() {
  if [[ "$SYSTEM_SECURITY_CHAOS" != "1" ]]; then
    skip_stage \
      "security-chaos" \
      "disabled-by-default (set SYSTEM_SECURITY_CHAOS=1 to enable the heavyweight chaos suite)"
    return
  fi

  run_stage "security-chaos" "$ROOT_DIR/test/system/subagents/security-chaos-tester.sh"
}

cleanup() {
  local log_path="$STAGES_DIR/infra-down.log"
  log "system-runner: stage=infra-down start"
  set +e
  "$ROOT_DIR/test/system/subagents/infra-orchestrator.sh" down >"$log_path" 2>&1
  local code=$?
  set -e
  if [[ "$code" -eq 0 ]]; then
    log "system-runner: stage=infra-down pass"
    log_stage "infra-down" "pass" 0 "$log_path"
  else
    log "system-runner: stage=infra-down fail"
    log_stage "infra-down" "fail" 0 "$log_path"
    any_failed=1
  fi

  go run "$ROOT_DIR/cmd/systemtest" junit \
    --results "$STAGE_RESULTS" \
    --suite "system-tests-${MODE}" \
    --out "$ARTIFACT_DIR/functional-junit.xml" >/dev/null 2>&1 || true

  go run "$ROOT_DIR/cmd/systemtest" run-result \
    --results "$STAGE_RESULTS" \
    --mode "$MODE" \
    --artifact-dir "$ARTIFACT_DIR" \
    --out "$ARTIFACT_DIR/run-result.json" >/dev/null 2>&1 || true
}

trap cleanup EXIT

go run "$ROOT_DIR/cmd/systemtest" validate-manifest \
  --manifest "$ROOT_DIR/test/system/screen-manifest.json" \
  --baseline-map "$ROOT_DIR/test/system/visual-baseline-map.json" \
  --app-dir "$ROOT_DIR/web/src/app"

go run "$ROOT_DIR/cmd/systemtest" matrix \
  --manifest "$ROOT_DIR/test/system/screen-manifest.json" \
  --format csv \
  --out "$ARTIFACT_DIR/coverage-matrix.csv"

run_stage "infra-up" "$ROOT_DIR/test/system/subagents/infra-orchestrator.sh" up "$ENV_REPORT"

if [[ ! -f "$ENV_REPORT" ]]; then
  echo "system-runner: missing env-report after infra-up: $ENV_REPORT" >&2
  exit 1
fi

load_env_report "$ENV_REPORT"

if [[ "$MODE" == "nightly" ]]; then
  run_stage "api-contract" "$ROOT_DIR/test/system/subagents/api-contract-tester.sh"
  run_ui_flow_stage
  run_ui_onboarding_auth_guard_stage
  run_ui_template_type_slug_edit_stage
  run_ui_workspace_management_stage
  run_ui_injector_flow_stage
  run_ui_adapter_sharing_stage
  run_visual_stage
  run_stage "ui-a11y" "$ROOT_DIR/test/system/subagents/ui-a11y-tester.sh"
  run_security_chaos_stage
else
  run_ui_flow_stage
  run_ui_onboarding_auth_guard_stage
  run_ui_template_type_slug_edit_stage
  run_ui_workspace_management_stage
  run_ui_injector_flow_stage
  run_ui_adapter_sharing_stage
  run_visual_stage
  skip_stage "ui-a11y" "nightly-only"
  run_stage "api-contract" "$ROOT_DIR/test/system/subagents/api-contract-tester.sh"
  skip_stage "security-chaos" "nightly-only"
fi

if [[ "$any_failed" -ne 0 ]]; then
  log "system-runner: completed with failures (artifact_dir=$ARTIFACT_DIR)"
  exit 1
fi

log "system-runner: completed successfully (artifact_dir=$ARTIFACT_DIR)"
