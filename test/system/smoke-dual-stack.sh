#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
ARTIFACT_ROOT="${ARTIFACT_ROOT:-$ROOT_DIR/artifacts/system/smoke-dual-stack}"
RUN_A_ROOT="$ARTIFACT_ROOT/run-a"
RUN_B_ROOT="$ARTIFACT_ROOT/run-b"
REPORT_A="$RUN_A_ROOT/env-report.json"
REPORT_B="$RUN_B_ROOT/env-report.json"
LOG_A="$RUN_A_ROOT/stack-up.log"
LOG_B="$RUN_B_ROOT/stack-up.log"

mkdir -p "$RUN_A_ROOT" "$RUN_B_ROOT"

cleanup() {
  set +e
  if [[ "${cleanup_done:-0}" != "1" && -f "$REPORT_A" ]]; then
    go run ./cmd/systemtest stack down --out "$REPORT_A" >/dev/null 2>&1 || true
  fi
  if [[ "${cleanup_done:-0}" != "1" && -f "$REPORT_B" ]]; then
    go run ./cmd/systemtest stack down --out "$REPORT_B" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

run_stack() {
  local report_path="$1"
  local spec="$2"
  local worktree="$3"
  local run_id="$4"
  local log_path="$5"

  (
    cd "$ROOT_DIR"
    go run ./cmd/systemtest stack up \
      --mode pr \
      --scope-spec "$spec" \
      --scope-worktree "$worktree" \
      --scope-run "$run_id" \
      --out "$report_path"
  ) >"$log_path" 2>&1
}

run_stack "$REPORT_A" "dual-stack" "spec-autonomous-e2e-isolation-a" "run-a" "$LOG_A" &
pid_a=$!
run_stack "$REPORT_B" "dual-stack" "spec-autonomous-e2e-isolation-b" "run-b" "$LOG_B" &
pid_b=$!

wait "$pid_a"
wait "$pid_b"

for report in "$REPORT_A" "$REPORT_B"; do
  [[ -f "$report" ]] || { echo "missing report: $report" >&2; exit 1; }
done

artifact_dir_a="$(jq -r '.runtime.artifact_dir' "$REPORT_A")"
artifact_dir_b="$(jq -r '.runtime.artifact_dir' "$REPORT_B")"
[[ "$artifact_dir_a" == "$RUN_A_ROOT" ]] || { echo "artifact_dir mismatch for run-a: $artifact_dir_a" >&2; exit 1; }
[[ "$artifact_dir_b" == "$RUN_B_ROOT" ]] || { echo "artifact_dir mismatch for run-b: $artifact_dir_b" >&2; exit 1; }

network_a="$(jq -r '.runtime.network' "$REPORT_A")"
network_b="$(jq -r '.runtime.network' "$REPORT_B")"
[[ -n "$network_a" && -n "$network_b" ]] || { echo "empty network in reports" >&2; exit 1; }
[[ "$network_a" != "$network_b" ]] || { echo "network collision: $network_a" >&2; exit 1; }

containers_a="$(jq -r '.runtime.containers | to_entries[] | "\(.key)=\(.value)"' "$REPORT_A" | sort)"
containers_b="$(jq -r '.runtime.containers | to_entries[] | "\(.key)=\(.value)"' "$REPORT_B" | sort)"
if [[ "$containers_a" == "$containers_b" ]]; then
  echo "container collision across runs" >&2
  exit 1
fi

curl -fsS "$(jq -r '.services.senda' "$REPORT_A")/health" >/dev/null
curl -fsS "$(jq -r '.services.senda' "$REPORT_B")/health" >/dev/null
curl -fsS "$(jq -r '.services.mailpit' "$REPORT_A")/api/v1/messages" >/dev/null
curl -fsS "$(jq -r '.services.mailpit' "$REPORT_B")/api/v1/messages" >/dev/null
curl -fsS "$(jq -r '.services.keycloak' "$REPORT_A")/realms/senda/.well-known/openid-configuration" >/dev/null
curl -fsS "$(jq -r '.services.keycloak' "$REPORT_B")/realms/senda/.well-known/openid-configuration" >/dev/null

go run ./cmd/systemtest stack down --out "$REPORT_A"
go run ./cmd/systemtest stack down --out "$REPORT_B"

docker network inspect "$network_a" >/dev/null 2>&1 && { echo "network cleanup failed for $network_a" >&2; exit 1; }
docker network inspect "$network_b" >/dev/null 2>&1 && { echo "network cleanup failed for $network_b" >&2; exit 1; }

while read -r container; do
  docker container inspect "$container" >/dev/null 2>&1 && { echo "container cleanup failed for $container" >&2; exit 1; }
done < <(jq -r '.runtime.containers | to_entries[] | .value' "$REPORT_A")
while read -r container; do
  docker container inspect "$container" >/dev/null 2>&1 && { echo "container cleanup failed for $container" >&2; exit 1; }
done < <(jq -r '.runtime.containers | to_entries[] | .value' "$REPORT_B")

cleanup_done=1

printf 'dual-stack smoke ok\n'
