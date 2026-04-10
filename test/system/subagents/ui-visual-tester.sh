#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=test/system/subagents/lib.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd go
require_cmd jq
require_cmd agent-browser
require_cmd timeout

load_env_report "$ENV_REPORT_FILE"

REPORT_HTML="$ARTIFACT_DIR/visual-diff-report.html"
REPORT_JSON="$ARTIFACT_DIR/visual-diff-report.json"
REPORT_MD="$ARTIFACT_DIR/visual-diff-report.md"
CASE_RESULTS="$ARTIFACT_DIR/ui-visual-cases.tsv"
ACTUAL_DIR="$ARTIFACT_DIR/ui-visual/actual"
TRACE_DIR="$ARTIFACT_DIR/ui-visual/traces"
STATE_DIR="$ARTIFACT_DIR/ui-visual/state"

BASELINE_MAP="$ROOT_DIR/test/system/visual-baseline-map.json"
MAP_IN_USE="$BASELINE_MAP"
if [[ "$SYSTEM_MODE" == "pr" ]]; then
  MAP_IN_USE="$ARTIFACT_DIR/visual-baseline-map.pr.json"
  jq '{version: .version, entries: [.entries[] | select(.critical == true)]}' "$BASELINE_MAP" >"$MAP_IN_USE"
fi

mkdir -p "$ACTUAL_DIR" "$TRACE_DIR" "$STATE_DIR"

capture_trace="${VISUAL_CAPTURE_TRACE:-}"
if [[ -z "$capture_trace" ]]; then
  capture_trace="0"
fi

session_namespace="$(basename "$ARTIFACT_DIR" | tr -cs '[:alnum:]' '-')"

ab() {
  local session="$1"
  shift
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$session" "$@"
}

ab_json() {
  local session="$1"
  shift
  timeout "${AGENT_BROWSER_TIMEOUT:-45s}" agent-browser --session "$session" "$@" --json
}

session_for_role() {
  local role="$1"
  local slug="${role//[^a-zA-Z0-9]/-}"
  echo "senda-visual-${session_namespace}-${slug}"
}

choose_role_for_route() {
  local route="$1"
  local scope="$2"

  case "$route" in
    "/"|"/login"|"/onboarding"|"/access-denied")
      echo "anonymous"
      return
      ;;
  esac

  case "$scope" in
    global) echo "superadmin" ;;
    tenant) echo "tenant-admin" ;;
    workspace) echo "workspace-admin" ;;
    *) echo "superadmin" ;;
  esac
}

route_scope() {
  local route="$1"
  case "$route" in
    /global*) echo "global" ;;
    /t/*/w/*) echo "workspace" ;;
    /t/*) echo "tenant" ;;
    *) echo "public" ;;
  esac
}

ensure_role_login() {
  local role="$1"
  if [[ "$role" == "anonymous" ]]; then
    return 0
  fi

  local session
  session="$(session_for_role "$role")"
  local state_file="$STATE_DIR/${session}.json"
  local marker_file="$STATE_DIR/${session}.done"
  local email
  email="$(role_email "$role")"
  local password
  password="$(role_password "$role")"

  if [[ -f "$marker_file" ]]; then
    return 0
  fi

  if [[ -f "$state_file" ]]; then
    ab "$session" state load "$state_file" >/dev/null
    touch "$marker_file"
    return 0
  fi

  log "ui-visual-tester: login role=$role email=$email"
  timeout 5s agent-browser --session "$session" close >/dev/null 2>&1 || true
  ab "$session" open "$FRONTEND_BASE_URL/login" >/dev/null
  ab "$session" wait 1200 >/dev/null
  local current_url
  local login_started=0
  for _ in $(seq 1 10); do
    current_url="$(ab_json "$session" get url | jq -r '.data.url // ""')"
    if [[ "$current_url" != *"/login"* ]]; then
      login_started=1
      break
    fi

    if ! ab_json "$session" eval '(() => {
      const buttons = Array.from(document.querySelectorAll("button"));
      const button = buttons.find((candidate) => {
        const text = (candidate.textContent || "").replace(/\s+/g, " ").trim();
        return /sign in|oidc|iniciar|ingresar/i.test(text);
      });
      if (!button) return "missing";
      button.click();
      return "clicked";
    })()' | jq -e '.data.result == "clicked"' >/dev/null; then
      echo "visual login button not found on frontend login page for role=$role" >&2
      return 1
    fi

    for _ in $(seq 1 15); do
      current_url="$(ab_json "$session" get url | jq -r '.data.url // ""')"
      if [[ "$current_url" != *"/login"* ]]; then
        login_started=1
        break
      fi
      sleep 1
    done

    if [[ "$login_started" -eq 1 ]]; then
      break
    fi
  done

  if [[ "$login_started" -ne 1 ]]; then
    echo "visual login did not leave frontend login page for role=$role last_url=$current_url" >&2
    return 1
  fi

  ab "$session" wait "#username" >/dev/null
  ab "$session" fill "#username" "$email" >/dev/null
  ab "$session" fill "#password" "$password" >/dev/null
  ab "$session" eval "(function(){var f=document.querySelector('#kc-form-login'); if (f) { f.submit(); return 'submitted'; } var b=document.querySelector('#kc-login'); if (b) { b.click(); return 'clicked'; } return 'missing'; })()" >/dev/null

  local settled=0
  for _ in $(seq 1 40); do
    current_url="$(ab_json "$session" get url | jq -r '.data.url // ""')"
    if [[ "$current_url" == "$FRONTEND_BASE_URL"* ]] && [[ "$current_url" != *"/api/auth/"* ]]; then
      settled=1
      break
    fi
    sleep 1
  done
  if [[ "$settled" -ne 1 ]]; then
    echo "visual login did not return to frontend for role=$role last_url=$current_url" >&2
    return 1
  fi
  ab "$session" state save "$state_file" >/dev/null
  touch "$marker_file"
}

close_sessions() {
  local role
  for role in anonymous superadmin tenant-admin workspace-admin; do
    local session
    session="$(session_for_role "$role")"
    timeout 5s agent-browser --session "$session" close >/dev/null 2>&1 || true
  done
}

trap close_sessions EXIT

start_frontend
ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships

if [[ "$SYSTEM_MODE" == "pr" ]]; then
  locales=(en)
  viewports=(desktop)
else
  locales=(en es)
  viewports=(desktop mobile)
fi
max_cases="${VISUAL_MAX_CASES:-0}"
case_count=0

echo -e "route\tscope\trole\tlocale\tviewport\tstatus\tnote\tscreenshot\ttrace" >"$CASE_RESULTS"

while IFS=$'\t' read -r route _critical; do
  scope="$(route_scope "$route")"
  role="$(choose_role_for_route "$route" "$scope")"
  for locale in "${locales[@]}"; do
    for viewport in "${viewports[@]}"; do
      if [[ "$max_cases" -gt 0 && "$case_count" -ge "$max_cases" ]]; then
        break 3
      fi
      case_count=$((case_count + 1))
      session="$(session_for_role "$role")"
      route_slug="$(sanitize_route "$route")"
      resolved_route="$(resolve_route "$route")"
      target_url="${FRONTEND_BASE_URL}${resolved_route}"
      screenshot_path="$ACTUAL_DIR/${route_slug}.${viewport}.${locale}.png"
      trace_path="$TRACE_DIR/${route_slug}.${viewport}.${locale}.zip"
      status="pass"
      note=""

      if ! ensure_role_login "$role"; then
        status="fail"
        note="login-failed"
      fi

      if [[ "$status" == "pass" ]]; then
        read -r viewport_w viewport_h <<<"$(viewport_size "$viewport")"
        if ! ab "$session" set viewport "$viewport_w" "$viewport_h" >/dev/null; then
          status="fail"
          note="set-viewport-failed"
        fi
      fi

      if [[ "$status" == "pass" ]]; then
        if [[ "$capture_trace" == "1" ]]; then
          ab "$session" trace start >/dev/null || true
        fi
        if ! ab "$session" open "$FRONTEND_BASE_URL/login" >/dev/null; then
          status="fail"
          note="open-login-failed"
        fi
      fi

      if [[ "$status" == "pass" ]]; then
        if ! ab "$session" eval "document.cookie='locale=${locale}; path=/; max-age=31536000; samesite=lax'; 'ok'" >/dev/null; then
          status="fail"
          note="set-locale-cookie-failed"
        fi
      fi

      if [[ "$status" == "pass" ]]; then
        if ! ab "$session" open "$target_url" >/dev/null; then
          status="fail"
          note="open-route-failed"
        fi
      fi

      if [[ "$status" == "pass" ]]; then
        ab "$session" wait 1200 >/dev/null || true
        if ! ab "$session" screenshot "$screenshot_path" >/dev/null; then
          status="fail"
          note="screenshot-failed"
        fi
      fi

      if [[ "$capture_trace" == "1" ]]; then
        ab "$session" trace stop "$trace_path" >/dev/null || true
      fi

      echo -e "${route}\t${scope}\t${role}\t${locale}\t${viewport}\t${status}\t${note}\t${screenshot_path}\t${trace_path}" >>"$CASE_RESULTS"
      if [[ "$status" != "pass" ]]; then
        echo "visual capture failed route=$route locale=$locale viewport=$viewport role=$role note=$note" >&2
      fi
    done
  done
done < <(jq -r '.entries[] | [.route, (.critical|tostring)] | @tsv' "$MAP_IN_USE")

allow_missing="false"
if [[ "$SYSTEM_MODE" == "pr" ]]; then
  allow_missing="true"
fi

log "ui-visual-tester: running visual diff"
systemtest visual-diff \
  --actual-dir "$ACTUAL_DIR" \
  --golden-dir "$ROOT_DIR/test/system/baselines/golden" \
  --pencil-dir "$ROOT_DIR/test/system/baselines/pencil" \
  --baseline-map "$MAP_IN_USE" \
  --locales "$(IFS=,; echo "${locales[*]}")" \
  --viewports "$(IFS=,; echo "${viewports[*]}")" \
  --critical-threshold 0.5 \
  --default-threshold 1.5 \
  --allow-missing-baselines="$allow_missing" \
  --out-html "$REPORT_HTML" \
  --out-json "$REPORT_JSON"

total_entries="$(jq 'length' "$REPORT_JSON")"
failed_entries="$(jq '[.[] | select(.status=="fail")] | length' "$REPORT_JSON")"
skipped_entries="$(jq '[.[] | select(.status=="skip")] | length' "$REPORT_JSON")"

cat >"$REPORT_MD" <<EOF_MD
# Visual Diff Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Total comparisons: $total_entries
- Failed comparisons: $failed_entries
- Skipped comparisons: $skipped_entries
- HTML report: \`$REPORT_HTML\`
- JSON report: \`$REPORT_JSON\`
- Capture cases: \`$CASE_RESULTS\`
EOF_MD

log "ui-visual-tester: reports written -> $REPORT_MD / $REPORT_HTML"
