# MJML Pre-Submit Validator — Design

**Date:** 2026-04-29
**Status:** Approved (brainstorming) — pending implementation plan
**Owner:** rendis

## Problem

An agent operating Senda via Claude Code authored a template body that wrapped MJML inside an HTML document scaffold (`<!DOCTYPE html>`, `<html>`, `<head>`, `<body>`) placed inside `<mj-raw>`. The save succeeded, but the runtime browser/MJML-XML parser failed:

> XML Parsing Error: not well-formed
> Line Number 3, Column 3

Root cause: the agent did not internalize that MJML *compiles into* HTML, so wrapping MJML in HTML is double-wrapping. The skill (`skills/senda/references/building-a-template.md`) lists `mj-raw` as a supported tag without an explicit anti-pattern, and the agent inferred (incorrectly) that `mj-raw` accepts a full HTML document.

There is no enforcement gate between "agent composes MJML" and "agent submits via API". Senda's `POST .../preview-mjml` would have caught the problem, but the skill treats it as recommended, not mandatory.

## Goal

Make the reported class of error impossible to commit by adding (a) a deterministic pre-submit check the agent must run, and (b) explicit anti-pattern documentation. Solution targets Claude Code agents with bash; other runtimes (MCP-only) can mirror the rules from the same source.

Non-goals: redesigning the MJML authoring flow, replacing `preview-mjml`, validating Senda variable syntax, building a YAML/JSON DSL.

## Approach

**B + D from brainstorming**: a static validator script + skill doc updates. Static-only — no HTTP, no auth, no env vars. Complements (does not replace) `preview-mjml`.

## Components

### 1. `skills/senda/scripts/mjml-check.sh`

- Pure bash, executable (`chmod +x`), no external deps beyond POSIX shell utilities (`grep`, `sed`, `awk`) available on macOS + Linux.
- CLI: `mjml-check.sh <path>` or `mjml-check.sh -` (stdin).
- Exit 0 if all rules pass; exit 1 if any rule fails. All violations reported in one run (no first-fail).
- All output to stderr. No stdout output on success.

**Rules (all hard-fail):**

| # | Rule | Message stem |
|---|---|---|
| 1 | No `<!DOCTYPE` (case-insensitive, anywhere) | `forbidden HTML document tag <!DOCTYPE` |
| 2 | No `<html`, `</html>` (case-insensitive) | `forbidden HTML root tag <html>` |
| 3 | No `<head>`, `</head>` (case-insensitive) | `forbidden HTML <head> tag (use <mj-head>)` |
| 4 | No `<body>`, `</body>` (case-insensitive — must distinguish from `<mj-body>`) | `forbidden HTML <body> tag (use <mj-body>)` |
| 5 | No `<meta`, `<title`, `<link`, `<base` (case-insensitive) | `forbidden HTML head tag <X>` |
| 6 | Document root is `<mjml>...</mjml>` (whitespace/comments allowed before) | `document must start with <mjml> and end with </mjml>` |
| 7 | If a violation of rules 1–5 is found inside an `<mj-raw>...</mj-raw>` block, append a clarifying hint pointing at the documented anti-pattern | `<mj-raw> is for small HTML snippets, not full documents` |

**Output format (stderr):**
```
mjml-check: FAIL line 3: forbidden HTML document tag <!DOCTYPE
  MJML compiles INTO HTML. Wrapping MJML in HTML is double-wrapping.
  See skills/senda/references/building-a-template.md "Anti-pattern: HTML wrappers".
mjml-check: FAIL line 4: forbidden HTML root tag <html>
  ...
mjml-check: 2 violation(s)
```

On success: silent, exit 0.

**Implementation notes:**
- Use `grep -niE` for line numbers and case-insensitive matching.
- Distinguish `<body>` from `<mj-body>` with a negative-lookbehind-equivalent regex (e.g., `(^|[^-])<body[> ]`).
- For the `<mj-raw>` clarifier, do a second pass: find `<mj-raw>...</mj-raw>` ranges (line-based; multi-line OK with `awk` or `sed -n`) and check if any rule-1–5 hit lands inside one.

### 2. `skills/senda/scripts/mjml-check.test.sh`

- Bash test runner. Executable.
- Embeds fixtures as heredocs (no separate fixture files — keeps the skill self-contained).
- Runs `mjml-check.sh` against each fixture and asserts expected exit code.

**Cases:**

| ID | Type | Fixture | Expected |
|---|---|---|---|
| ok-1 | PASS | minimal `<mjml><mj-body><mj-section><mj-column><mj-text>hi</mj-text>...` | exit 0 |
| ok-2 | PASS | hero + section + button | exit 0 |
| ok-3 | PASS | `<mj-raw><div class="x">snippet</div></mj-raw>` (legitimate small HTML) | exit 0 |
| fail-1 | FAIL | `<!DOCTYPE html>` at top | exit 1, stderr mentions DOCTYPE |
| fail-2 | FAIL | `<html><head>...</head><body>...</body></html>` outside any `<mjml>` | exit 1, multiple violations |
| fail-3 | FAIL | `<mj-raw><!DOCTYPE html><html>...</html></mj-raw>` (the reported case) | exit 1, mentions `<mj-raw>` clarifier |
| fail-4 | FAIL | `<head><meta charset="utf-8"></head>` inside body | exit 1 |
| fail-5 | FAIL | document does not start with `<mjml>` | exit 1, rule 6 |

Runner output: `OK <case>` / `FAIL <case>: <reason>`, exit 1 if any case fails.

### 3. Skill doc updates

**`skills/senda/references/building-a-template.md`:**
- Insert new section "Anti-pattern: HTML wrappers" immediately after "Document skeleton". Show wrong example (the reported one) and right example. Two paragraphs.
- Update "Composer's checklist" — replace the current step about `preview-mjml` with two ordered steps: (1) `bash skills/senda/scripts/mjml-check.sh <file>` (mandatory, blocks submit), (2) `POST .../preview-mjml` (mandatory before publish).
- Update the "Engine and version" note about `mj-raw` to add: *"`<mj-raw>` accepts small HTML snippets, never full documents — see Anti-pattern below."*

**`skills/senda/references/versions-locales-and-builder.md`:**
- Add a Gotcha bullet: *"Pre-submit gate: run `skills/senda/scripts/mjml-check.sh` on `body_mjml` before any version POST/PUT — blocks the HTML-wrapper class of error that `preview-mjml` would otherwise catch only on send."*

**`skills/senda/SKILL.md`:**
- Add one row to the decision table: *"If you change MJML composition rules → update `skills/senda/scripts/mjml-check.sh` (rules + tests)."*

## Data flow

```
agent → composes MJML → mjml-check.sh (static) → preview-mjml (compile) → POST/PUT version/locale
                                ↓ exit 1
                        agent fixes, retries
```

The validator never calls Senda. `preview-mjml` is a separate, downstream step the skill documents.

## Error handling

- Script: exits 1 on any rule violation. All violations printed before exit (no first-fail). On script bugs (unexpected grep failure, IO error), exit 2 with `mjml-check: internal error: <msg>` to distinguish from rule failures.
- Tests: run all cases; report each; exit 1 if any case fails. No partial-credit.

## Testing

- Test script lives next to the validator (`mjml-check.test.sh`).
- Run manually during development.
- Not wired into `make` targets — YAGNI; can be added later if the script gains complexity.
- The skill update in `SKILL.md` makes maintenance of the test fixtures part of the maintenance contract.

## Out of scope

- HTTP / `preview-mjml` integration in the script. Kept separate so the script has zero deps and zero auth surface.
- Variable-syntax validation (`{{ event.x }}` / `{{ injector.x.y }}`). Skill already documents these are silent — adding validation here duplicates without value.
- A CLI to *generate* MJML (rejected during brainstorming as a YAML→MJML translation that adds an extra format without removing the underlying error class).
- MCP-only runtime support. The script is bash; MCP-only clients without shell access don't benefit. If that becomes a real need later, the rules can be ported to a server-side endpoint or an MCP tool — but the static rules in the script are the canonical source of truth either way.

## Maintenance contract

When the visual builder or `gomjml` upgrades change which top-level tags are legal MJML, update `mjml-check.sh` in the same PR. The `SKILL.md` decision-table entry enforces this as a review checklist.

## Acceptance criteria

1. The reported case (`<mj-raw><!DOCTYPE html><html>...</html></mj-raw>`) piped into `bash skills/senda/scripts/mjml-check.sh -` exits 1 and the stderr output includes the `<mj-raw>` clarifier message.
2. `bash skills/senda/scripts/mjml-check.test.sh` exits 0 with all 8 cases (3 PASS, 5 FAIL) passing — fixtures are embedded as heredocs in the test runner, no separate fixture files.
3. `skills/senda/references/building-a-template.md` contains the new "Anti-pattern: HTML wrappers" section and the updated checklist references the script.
4. A fresh agent reading the skill end-to-end after the change cannot infer that `<mj-raw>` accepts a full HTML document.
