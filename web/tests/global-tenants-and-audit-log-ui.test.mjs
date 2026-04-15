import test from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { repoRoot } from "./test-root.mjs";

const root = repoRoot;

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("audit log filters share a responsive labeled grid layout", () => {
  const filters = read("web/src/components/audit-log/audit-log-filters.tsx");

  assert.match(
    filters,
    /grid w-full gap-3/,
    "Audit log filters should use a stable grid layout instead of ad-hoc flex alignment",
  );

  assert.match(
    filters,
    /<FilterField label="Search"/,
    "Search input should participate in the same labeled field structure as the other filters",
  );
});

test("global tenants management screen is wired into navigation and routing", () => {
  assert.equal(
    existsSync(join(root, "web/src/app/(dashboard)/global/tenants/page.tsx")),
    true,
    "Global tenants route should exist",
  );

  assert.equal(
    existsSync(join(root, "web/src/components/tenants/tenants-content.tsx")),
    true,
    "Tenants content feature component should exist",
  );

  const sidebar = read("web/src/components/layout/sidebar.tsx");
  const header = read("web/src/components/layout/header.tsx");
  const en = read("web/messages/en.json");
  const es = read("web/messages/es.json");

  assert.match(sidebar, /t\("tenants"\)/, "Sidebar should expose a Tenants navigation item");
  assert.match(header, /"\/tenants": t\("tenants"\)/, "Header title mapping should resolve the Tenants page");
  assert.match(en, /"tenants": "Tenants"/, "English translations should include nav.tenants");
  assert.match(es, /"tenants": "Tenants"|"tenants": "Inquilinos"/, "Spanish translations should include nav.tenants");
});

test("tenant management hooks and coverage metadata exist", () => {
  assert.equal(
    existsSync(join(root, "web/src/hooks/use-tenants-mgmt.ts")),
    true,
    "Tenant management hooks should exist",
  );

  const screenManifest = read("test/system/screen-manifest.json");
  const visualBaselineMap = read("test/system/visual-baseline-map.json");

  assert.match(screenManifest, /"route": "\/global\/tenants"/, "Screen manifest should cover /global/tenants");
  assert.match(visualBaselineMap, /"route": "\/global\/tenants"/, "Visual baseline map should cover /global/tenants");
});

test("tenant workspaces management screen is wired into navigation and coverage metadata", () => {
  assert.equal(
    existsSync(join(root, "web/src/app/(dashboard)/t/[tenantCode]/workspaces/page.tsx")),
    true,
    "Tenant workspaces route should exist",
  );

  assert.equal(
    existsSync(join(root, "web/src/components/workspaces/workspaces-content.tsx")),
    true,
    "Workspaces content feature component should exist",
  );

  assert.equal(
    existsSync(join(root, "web/src/hooks/use-workspaces-mgmt.ts")),
    true,
    "Workspace management hooks should exist",
  );

  const sidebar = read("web/src/components/layout/sidebar.tsx");
  const header = read("web/src/components/layout/header.tsx");
  const switcher = read("web/src/components/shared/scope-switcher.tsx");
  const en = read("web/messages/en.json");
  const es = read("web/messages/es.json");
  const screenManifest = read("test/system/screen-manifest.json");
  const visualBaselineMap = read("test/system/visual-baseline-map.json");

  assert.match(sidebar, /t\("workspaces"\)/, "Sidebar should expose a Workspaces navigation item");
  assert.match(header, /"\/workspaces": t\("workspaces"\)/, "Header title mapping should resolve the Workspaces page");
  assert.match(switcher, /manageWorkspaces|Create Workspace|\/workspaces/, "Scope switcher should expose a visible workspace management entrypoint");
  assert.match(en, /"workspaces": "Workspaces"/, "English translations should include nav.workspaces");
  assert.match(es, /"workspaces": "Workspaces"|"workspaces": "Espacios de trabajo"/, "Spanish translations should include nav.workspaces");
  assert.match(screenManifest, /"route": "\/t\/\[tenantCode\]\/workspaces"/, "Screen manifest should cover /t/[tenantCode]/workspaces");
  assert.match(visualBaselineMap, /"route": "\/t\/\[tenantCode\]\/workspaces"/, "Visual baseline map should cover /t/[tenantCode]/workspaces");
});

test("tenant workspaces list exposes a confirmed status toggle in the table", () => {
  const source = read("web/src/components/workspaces/workspaces-content.tsx");

  assert.match(
    source,
    /role="switch"/,
    "Workspace rows should expose a switch-like control directly in the status column",
  );

  assert.match(
    source,
    /aria-label=\{`Toggle workspace \$\{displayCode\} status`\}/,
    "Workspace status toggle should expose an accessible display-code label",
  );

  assert.match(
    source,
    /"Disable workspace"|"Enable workspace"/,
    "Changing workspace status from the list should require an explicit confirmation dialog",
  );
});
