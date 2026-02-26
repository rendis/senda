#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd go
require_cmd jq
require_cmd agent-browser
require_cmd timeout

REPORT_PATH="$ARTIFACT_DIR/a11y-report.md"
RESULTS_JSON="$ARTIFACT_DIR/a11y-results.json"
CASE_RESULTS="$ARTIFACT_DIR/a11y-cases.tsv"
MATRIX_TSV="$ARTIFACT_DIR/a11y-matrix.tsv"
FILTERED_TSV="$ARTIFACT_DIR/a11y-matrix.filtered.tsv"
RAW_DIR="$ARTIFACT_DIR/a11y/raw"
STATE_DIR="$ARTIFACT_DIR/a11y/state"

mkdir -p "$RAW_DIR" "$STATE_DIR"

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
  echo "senda-a11y-${slug}"
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
    ab "$session" state load "$state_file" >/dev/null
    touch "$marker_file"
    return 0
  fi

  log "ui-a11y-tester: login role=$role email=$email"
  ab "$session" open "$FRONTEND_BASE_URL/login" >/dev/null
  ab "$session" wait 1200 >/dev/null

  local current_url
  current_url="$(ab_json "$session" get url | jq -r '.data.url // ""')"
  if [[ "$current_url" == *"/login"* ]]; then
    ab "$session" click "button" >/dev/null
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
    echo "a11y login did not return to frontend for role=$role last_url=$current_url" >&2
    return 1
  fi

  ab "$session" state save "$state_file" >/dev/null
  touch "$marker_file"
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

A11Y_JS=$(cat <<'EOF_JS'
(async () => {
  if (!window.axe) {
    await new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = "https://cdnjs.cloudflare.com/ajax/libs/axe-core/4.10.2/axe.min.js";
      script.onload = resolve;
      script.onerror = reject;
      document.head.appendChild(script);
    });
  }

  const isVisible = (el) => {
    const style = window.getComputedStyle(el);
    return style.display !== "none" && style.visibility !== "hidden" && style.opacity !== "0";
  };

  const hasAccessibleName = (el) => {
    const text = (el.innerText || "").trim();
    const aria = (el.getAttribute("aria-label") || "").trim();
    const labelledBy = (el.getAttribute("aria-labelledby") || "").trim();
    const title = (el.getAttribute("title") || "").trim();
    return Boolean(text || aria || labelledBy || title);
  };

  const controls = Array.from(document.querySelectorAll("input,select,textarea"))
    .filter((el) => el.getAttribute("type") !== "hidden")
    .filter(isVisible);
  const unlabeledControls = controls.filter((el) => {
    const id = el.getAttribute("id");
    const hasLabel = id ? document.querySelector(`label[for="${id}"]`) : null;
    const aria = el.getAttribute("aria-label") || el.getAttribute("aria-labelledby");
    return !hasLabel && !aria;
  }).length;

  const buttonsWithoutName = Array.from(document.querySelectorAll("button,[role='button']"))
    .filter(isVisible)
    .filter((el) => !hasAccessibleName(el)).length;

  const linksWithoutName = Array.from(document.querySelectorAll("a"))
    .filter(isVisible)
    .filter((el) => !hasAccessibleName(el)).length;

  const imagesWithoutAlt = Array.from(document.querySelectorAll("img"))
    .filter((img) => !img.hasAttribute("alt") || (img.getAttribute("alt") || "").trim() === "").length;

  const landmarkIssues = [];
  if (!document.querySelector("main")) landmarkIssues.push("missing-main");
  if (!document.querySelector("nav")) landmarkIssues.push("missing-nav");
  if (!document.querySelector("header")) landmarkIssues.push("missing-header");

  const axe = await window.axe.run(document, {
    runOnly: {
      type: "tag",
      values: ["wcag2a", "wcag2aa"]
    }
  });

  const byImpact = {
    critical: axe.violations.filter((v) => v.impact === "critical").length,
    serious: axe.violations.filter((v) => v.impact === "serious").length,
    moderate: axe.violations.filter((v) => v.impact === "moderate").length,
    minor: axe.violations.filter((v) => v.impact === "minor").length
  };

  return {
    url: window.location.href,
    title: document.title,
    manual: {
      unlabeledControls,
      buttonsWithoutName,
      linksWithoutName,
      imagesWithoutAlt,
      landmarkIssues
    },
    axe: {
      totalViolations: axe.violations.length,
      byImpact,
      violations: axe.violations.map((v) => ({
        id: v.id,
        impact: v.impact,
        description: v.description,
        help: v.help,
        nodeCount: v.nodes.length
      }))
    }
  };
})()
EOF_JS
)

start_frontend
ensure_runtime_env
load_runtime_env
seed_keycloak_users
seed_rbac_memberships

go run "$ROOT_DIR/cmd/systemtest" matrix \
  --manifest "$ROOT_DIR/test/system/screen-manifest.json" \
  --format tsv \
  --out "$MATRIX_TSV"

if [[ "$SYSTEM_MODE" == "pr" ]]; then
  awk -F '\t' 'NR==1 || ($7=="true" && $4=="superadmin" && $5=="en" && $6=="desktop")' "$MATRIX_TSV" >"$FILTERED_TSV"
else
  cp "$MATRIX_TSV" "$FILTERED_TSV"
fi

echo -e "route\tscope\trole\tlocale\tviewport\tstatus\tcritical\tserious\tmanual_issues\traw_json" >"$CASE_RESULTS"
max_cases="${A11Y_MAX_CASES:-0}"
case_count=0

while IFS=$'\t' read -r route route_slug scope role locale viewport critical pencil_frame_id preconditions actions assertions; do
  if [[ "$route" == "route" ]]; then
    continue
  fi
  if [[ "$max_cases" -gt 0 && "$case_count" -ge "$max_cases" ]]; then
    break
  fi
  case_count=$((case_count + 1))

  session="$(session_for_role "$role")"
  raw_path="$RAW_DIR/${route_slug}.${viewport}.${locale}.${role}.json"
  status="pass"
  critical_count=0
  serious_count=0
  manual_issues=0

  if ! ensure_role_login "$role"; then
    status="fail"
  fi

  if [[ "$status" == "pass" ]]; then
    read -r viewport_w viewport_h <<<"$(viewport_size "$viewport")"
    if ! ab "$session" set viewport "$viewport_w" "$viewport_h" >/dev/null; then
      status="fail"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    if ! ab "$session" open "$FRONTEND_BASE_URL/login" >/dev/null; then
      status="fail"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    if ! ab "$session" eval "document.cookie='locale=${locale}; path=/; max-age=31536000; samesite=lax'; 'ok'" >/dev/null; then
      status="fail"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    target_url="${FRONTEND_BASE_URL}$(resolve_route "$route")"
    if ! ab "$session" open "$target_url" >/dev/null; then
      status="fail"
    fi
  fi

  if [[ "$status" == "pass" ]]; then
    ab "$session" wait 1200 >/dev/null || true
    if ! a11y_json="$(ab_json "$session" eval "$A11Y_JS")"; then
      status="fail"
    else
      echo "$a11y_json" | jq -c '.data.result' >"$raw_path"
      critical_count="$(jq -r '.axe.byImpact.critical // 0' "$raw_path")"
      serious_count="$(jq -r '.axe.byImpact.serious // 0' "$raw_path")"
      manual_issues="$(jq -r '(.manual.unlabeledControls // 0) + (.manual.buttonsWithoutName // 0) + (.manual.linksWithoutName // 0) + (.manual.imagesWithoutAlt // 0) + ((.manual.landmarkIssues // []) | length)' "$raw_path")"
      if [[ "$critical_count" -gt 0 || "$serious_count" -gt 0 || "$manual_issues" -gt 0 ]]; then
        status="fail"
      fi
    fi
  fi

  echo -e "${route}\t${scope}\t${role}\t${locale}\t${viewport}\t${status}\t${critical_count}\t${serious_count}\t${manual_issues}\t${raw_path}" >>"$CASE_RESULTS"
done <"$FILTERED_TSV"

jq -Rn --slurpfile rows <(awk -F '\t' 'NR>1 {printf("{\"route\":\"%s\",\"scope\":\"%s\",\"role\":\"%s\",\"locale\":\"%s\",\"viewport\":\"%s\",\"status\":\"%s\",\"critical\":%d,\"serious\":%d,\"manual_issues\":%d,\"raw\":\"%s\"}\n",$1,$2,$3,$4,$5,$6,$7,$8,$9,$10)}' "$CASE_RESULTS" | jq -s '.') '$rows[0]' >"$RESULTS_JSON"

total="$(awk 'NR>1 {count++} END {print count+0}' "$CASE_RESULTS")"
failed="$(awk -F '\t' 'NR>1 && $6=="fail" {count++} END {print count+0}' "$CASE_RESULTS")"

cat >"$REPORT_PATH" <<EOF_MD
# Accessibility Report

- Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)
- Mode: $SYSTEM_MODE
- Total cases: $total
- Failed cases: $failed
- Matrix: \`$FILTERED_TSV\`
- Case results: \`$CASE_RESULTS\`
- Raw JSON: \`$RESULTS_JSON\`

## Failure Cases
EOF_MD

{
  echo ""
  if [[ "$failed" -eq 0 ]]; then
    echo "- None"
  else
    awk -F '\t' 'NR>1 && $6=="fail" {printf("- route=%s role=%s locale=%s viewport=%s critical=%s serious=%s manual=%s\n",$1,$3,$4,$5,$7,$8,$9)}' "$CASE_RESULTS"
  fi
} >>"$REPORT_PATH"

log "ui-a11y-tester: report written -> $REPORT_PATH"
if [[ "$failed" -gt 0 ]]; then
  exit 1
fi
