#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd go
require_cmd jq
require_cmd agent-browser
require_cmd timeout

BODY_TEXT_JS='(() => {
  if (!document.body) return "";
  const excluded = new Set(["SCRIPT", "STYLE", "NOSCRIPT", "TEMPLATE"]);
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const parent = node.parentElement;
      if (!parent) return NodeFilter.FILTER_REJECT;
      if (excluded.has(parent.tagName)) return NodeFilter.FILTER_REJECT;
      const text = (node.textContent || "").replace(/\s+/g, " ").trim();
      if (!text) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    }
  });
  const parts = [];
  while (walker.nextNode()) {
    parts.push((walker.currentNode.textContent || "").replace(/\s+/g, " ").trim());
  }
  return parts.join(" ");
})()'

load_env_report "$ENV_REPORT_FILE"

REPORT_PATH="$ARTIFACT_DIR/ui-flow-report.md"
MATRIX_TSV="$ARTIFACT_DIR/ui-flow-matrix.tsv"
FILTERED_TSV="$ARTIFACT_DIR/ui-flow-matrix.filtered.tsv"
CASE_RESULTS="$ARTIFACT_DIR/ui-flow-cases.tsv"
SCREENSHOT_DIR="$ARTIFACT_DIR/ui-flow/screenshots"
TRACE_DIR="$ARTIFACT_DIR/ui-flow/traces"
STATE_DIR="$ARTIFACT_DIR/ui-flow/state"

mkdir -p "$SCREENSHOT_DIR" "$TRACE_DIR" "$STATE_DIR"

capture_trace="${UI_CAPTURE_TRACE:-}"
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
  echo "senda-${session_namespace}-${slug}"
}

sanitize_field() {
  local v="$1"
  v="${v//$'\t'/ }"
  v="${v//$'\n'/ }"
  v="${v//$'\r'/ }"
  echo "$v"
}

is_public_route() {
  local route="$1"
  case "$route" in
    "/"|"/login"|"/onboarding"|"/access-denied")
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

role_has_access() {
  local scope="$1"
  local role="$2"
  local route="$3"

  if is_public_route "$route"; then
    return 0
  fi

  case "$role" in
    superadmin)
      return 0
      ;;
    no-member)
      return 1
      ;;
    tenant-admin)
      [[ "$scope" == "tenant" || "$scope" == "workspace" ]]
      return
      ;;
    workspace-admin|workspace-editor|workspace-viewer)
      [[ "$scope" == "workspace" ]]
      return
      ;;
    *)
      return 1
      ;;
  esac
}

ensure_role_login() {
  local role="$1"
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
    log "ui-flow-tester: loading existing state role=$role"
    ab "$session" state load "$state_file" >/dev/null
    touch "$marker_file"
    return 0
  fi

  log "ui-flow-tester: login role=$role email=$email"
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
      echo "login button not found on frontend login page for role=$role" >&2
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
    echo "login did not leave frontend login page for role=$role last_url=$current_url" >&2
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
    echo "login did not return to frontend for role=$role last_url=$current_url" >&2
    return 1
  fi

  ab "$session" state save "$state_file" >/dev/null
  touch "$marker_file"
}

run_case() {
  local route="$1"
  local route_slug="$2"
  local scope="$3"
  local role="$4"
  local locale="$5"
  local viewport="$6"
  local critical="$7"

  local session
  session="$(session_for_role "$role")"
  local resolved_route
  resolved_route="$(resolve_route "$route")"
  local target_url="${FRONTEND_BASE_URL}${resolved_route}"
  local viewport_w viewport_h
  read -r viewport_w viewport_h <<<"$(viewport_size "$viewport")"

  local screenshot_path="$SCREENSHOT_DIR/${route_slug}.${viewport}.${locale}.${role}.png"
  local trace_path="$TRACE_DIR/${route_slug}.${viewport}.${locale}.${role}.zip"
  local final_url=""
  local body_text=""
  local status="pass"
  local note=""
  local expected="allow"

  if role_has_access "$scope" "$role" "$route"; then
    expected="allow"
  else
    expected="deny"
  fi

  if ! ensure_role_login "$role"; then
    status="fail"
    note="login-failed"
  fi

  if [[ "$status" == "pass" ]] && [[ "$capture_trace" == "1" ]]; then
    ab "$session" trace start >/dev/null || true
  fi

  if [[ "$status" == "pass" ]]; then
    if ! ab "$session" set viewport "$viewport_w" "$viewport_h" >/dev/null; then
      status="fail"
      note="set-viewport-failed"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
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
    ab "$session" wait 1500 >/dev/null || true
    final_url="$(ab_json "$session" get url | jq -r '.data.url // ""')"
    body_text="$(ab_json "$session" eval "$BODY_TEXT_JS" | jq -r '.data.result // ""')"
  fi

  if [[ "$status" == "pass" ]]; then
    if [[ "$expected" == "allow" ]]; then
      if ! is_public_route "$route"; then
        if [[ "$final_url" == *"/access-denied"* || "$final_url" == *"/login"* ]]; then
          status="fail"
          note="unexpected-redirect-${final_url}"
        fi
      fi
    else
      if [[ "$final_url" != *"/access-denied"* && "$final_url" != *"/login"* ]] && [[ "$body_text" != *"Access Denied"* ]] && [[ "$body_text" != *"Acceso Denegado"* ]]; then
        status="fail"
        note="expected-deny-but-page-open"
      fi
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    if [[ "$body_text" == *"Internal Server Error"* || "$body_text" == *"Unhandled Runtime Error"* || "$body_text" == *"This page could not be found"* ]]; then
      status="fail"
      note="runtime-or-404-error"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    ab "$session" screenshot "$screenshot_path" >/dev/null || true
  else
    ab "$session" screenshot "$screenshot_path" >/dev/null || true
  fi

  if [[ "$capture_trace" == "1" ]]; then
    ab "$session" trace stop "$trace_path" >/dev/null || true
  fi

  echo -e "${route}\t${scope}\t${role}\t${locale}\t${viewport}\t${critical}\t${expected}\t$(sanitize_field "$final_url")\t${status}\t$(sanitize_field "$note")\t${screenshot_path}\t${trace_path}" >>"$CASE_RESULTS"

  [[ "$status" == "pass" ]]
}

close_sessions() {
  local role
  for role in superadmin tenant-admin workspace-admin workspace-editor workspace-viewer no-member; do
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

log "ui-flow-tester: generating matrix"
go run "$ROOT_DIR/cmd/systemtest" matrix \
  --manifest "$ROOT_DIR/test/system/screen-manifest.json" \
  --format tsv \
  --out "$MATRIX_TSV"

if [[ "$SYSTEM_MODE" == "pr" ]]; then
  awk -F '\t' 'NR==1 || ($7=="true" && $5=="en" && $6=="desktop" && ($4=="superadmin" || $4=="no-member"))' "$MATRIX_TSV" >"$FILTERED_TSV"
else
  awk -F '\t' 'NR==1 || ($7=="true" && $5=="en" && $6=="desktop")' "$MATRIX_TSV" >"$FILTERED_TSV"
fi

echo -e "route\tscope\trole\tlocale\tviewport\tcritical\texpected\tfinal_url\tstatus\tnote\tscreenshot\ttrace" >"$CASE_RESULTS"

total=0
passed=0
failed=0
max_cases="${UI_MAX_CASES:-0}"

while IFS=$'\t' read -r route route_slug scope role locale viewport critical pencil_frame_id preconditions actions assertions; do
  if [[ "$route" == "route" ]]; then
    continue
  fi
  if [[ "$max_cases" -gt 0 && "$total" -ge "$max_cases" ]]; then
    break
  fi
  total=$((total + 1))
  if run_case "$route" "$route_slug" "$scope" "$role" "$locale" "$viewport" "$critical"; then
    passed=$((passed + 1))
  else
    failed=$((failed + 1))
  fi
done <"$FILTERED_TSV"

cat >"$REPORT_PATH" <<EOF_MD
# UI Flow Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Frontend URL: $FRONTEND_BASE_URL
- Total cases: $total
- Passed: $passed
- Failed: $failed
- Matrix source: \`$FILTERED_TSV\`
- Case results: \`$CASE_RESULTS\`

## Failure Cases
EOF_MD

{
  echo ""
  if [[ "$failed" -eq 0 ]]; then
    echo "- None"
  else
    awk -F '\t' 'NR>1 && $9=="fail" {printf("- route=%s role=%s locale=%s viewport=%s note=%s\n",$1,$3,$4,$5,$10)}' "$CASE_RESULTS"
  fi
} >>"$REPORT_PATH"

log "ui-flow-tester: report written -> $REPORT_PATH"
if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
