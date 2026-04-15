#!/usr/bin/env node

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import process from "node:process";

const rootUrl = new URL("..", import.meta.url);
const rootDir = fileURLToPath(rootUrl);

const read = (relativePath) => readFileSync(new URL(relativePath, rootUrl), "utf8");

const errors = [];

const fail = (file, message) => {
  errors.push(`${file}: ${message}`);
};

const requireMatch = (file, pattern, message) => {
  if (!pattern.test(read(file))) {
    fail(file, message);
  }
};

const forbidMatch = (file, pattern, message) => {
  if (pattern.test(read(file))) {
    fail(file, message);
  }
};

const requireJsonPath = (file, path, expected) => {
  const json = JSON.parse(read(file));
  const actual = path.reduce((value, key) => (value == null ? undefined : value[key]), json);
  if (actual !== expected) {
    fail(file, `expected ${path.join(".")}=${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
};

const requireWorkflowName = (file, fragment) => {
  requireMatch(file, new RegExp(`^name:\\s*.*${fragment}.*$`, "mi"), `workflow name must include ${fragment}`);
};

const ensureNoMainSemantics = (file) => {
  forbidMatch(file, /\bci-main\b/, "ci-main is no longer a public gate");
  forbidMatch(file, /\bci-backend-main\b/, "ci-backend-main is no longer a public gate");
};

const docs = [
  "README.md",
  "CONTRIBUTING.md",
  "docs/DEVELOPMENT.md",
  "docs/specs/TESTING_STRATEGY.md",
  "web/README.md",
];

for (const file of docs) {
  ensureNoMainSemantics(file);
}

requireMatch("Makefile", /^\.PHONY:.*\bci-taxonomy-check\b/m, "Add ci-taxonomy-check to .PHONY");
requireMatch("Makefile", /^ci-frontend:.*\n\tbash scripts\/run-github-gates\.sh frontend/m, "ci-frontend must delegate to the GitHub frontend gate script");
requireMatch("Makefile", /^ci-pr:/m, "ci-pr target must exist");
requireMatch("Makefile", /\$\(MAKE\) ci-taxonomy-check/, "ci-pr must run the taxonomy check");
requireMatch("Makefile", /bash scripts\/run-github-gates\.sh pr/, "ci-pr must still compose the PR gates");
requireMatch("Makefile", /system-pr: ## .*manual/i, "system-pr help text must call out that it is manual");
requireMatch("Makefile", /system-nightly: ## .*manual/i, "system-nightly help text must call out that it is manual");
ensureNoMainSemantics("Makefile");

requireMatch("scripts/run-github-gates.sh", /usage: \$0 <backend-pr\|frontend\|pr>/, "Usage text must advertise only the public PR modes");
requireMatch("scripts/run-github-gates.sh", /corepack pnpm --dir web test/, "Frontend gate must run the canonical test entrypoint");
forbidMatch("scripts/run-github-gates.sh", /\bbackend-main\b/, "backend-main mode is no longer public");
forbidMatch("scripts/run-github-gates.sh", /\bmain\b/, "main mode is no longer public");

requireJsonPath("web/package.json", ["scripts", "test"], "node --test");

requireMatch(".github/workflows/backend-gate.yml", /run: make ci-backend-pr/, "backend gate workflow must call ci-backend-pr");
requireMatch(".github/workflows/frontend-gate.yml", /run: make ci-frontend/, "frontend gate workflow must call ci-frontend");
requireMatch(".github/workflows/taxonomy-check.yml", /run: make ci-taxonomy-check/, "taxonomy workflow must run the taxonomy check");
requireMatch(".github/workflows/taxonomy-check.yml", /pull_request:/, "taxonomy workflow must run on pull_request");
requireWorkflowName(".github/workflows/system-pr-gate.yml", "Manual");
requireWorkflowName(".github/workflows/system-nightly-gate.yml", "Manual");
requireWorkflowName(".github/workflows/chaos-e2e.yml", "Observational");
requireMatch(".github/workflows/system-pr-gate.yml", /workflow_dispatch:/, "system-pr gate must remain manually triggered");
requireMatch(".github/workflows/system-nightly-gate.yml", /workflow_dispatch:/, "system-nightly gate must remain manually triggered");
requireMatch(".github/workflows/chaos-e2e.yml", /workflow_dispatch:/, "chaos E2E must remain manually triggered");

requireMatch("README.md", /make ci-taxonomy-check/, "README must document the taxonomy check");
requireMatch("README.md", /pnpm --dir web test/, "README must document the canonical frontend test entrypoint");
requireMatch("README.md", /manual\s*\/\s*observational|workflow_dispatch/i, "README must distinguish manual or observational system gates");

requireMatch("CONTRIBUTING.md", /make ci-taxonomy-check/, "CONTRIBUTING must document the taxonomy check");
requireMatch("CONTRIBUTING.md", /pnpm --dir web test/, "CONTRIBUTING must document the canonical frontend test entrypoint");
requireMatch("CONTRIBUTING.md", /manual\s*\/\s*observational|workflow_dispatch/i, "CONTRIBUTING must distinguish manual or observational system gates");

requireMatch("docs/DEVELOPMENT.md", /make ci-taxonomy-check/, "Development guide must document the taxonomy check");
requireMatch("docs/DEVELOPMENT.md", /pnpm --dir web test/, "Development guide must document the canonical frontend test entrypoint");
requireMatch("docs/DEVELOPMENT.md", /manual\s*\/\s*observational|workflow_dispatch/i, "Development guide must distinguish manual or observational system gates");

requireMatch("docs/specs/TESTING_STRATEGY.md", /pnpm --dir web test/, "Testing strategy must mention the canonical frontend test entrypoint");
requireMatch("docs/specs/TESTING_STRATEGY.md", /workflow_dispatch|manual\s*\/\s*observational/i, "Testing strategy must separate manual and automatic system gates");

requireMatch("web/README.md", /pnpm --dir web test/, "web/README must document the canonical frontend test entrypoint");

if (errors.length > 0) {
  console.error("CI taxonomy check failed:");
  for (const error of errors) {
    console.error(`- ${error}`);
  }
  process.exitCode = 1;
} else {
  console.log(`CI taxonomy check passed from ${rootDir}`);
}
