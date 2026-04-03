import test from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

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
