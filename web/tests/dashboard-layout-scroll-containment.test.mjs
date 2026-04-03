import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

test("dashboard shell constrains the app to the viewport and scrolls inside main", () => {
  const dashboardShell = read("web/src/components/layout/dashboard-shell.tsx");

  assert.match(
    dashboardShell,
    /className="flex h-svh overflow-hidden bg-page"/,
    "Dashboard shell must be locked to the viewport instead of extending the document height",
  );

  assert.match(
    dashboardShell,
    /className="flex min-h-0 min-w-0 flex-1 flex-col"/,
    "Dashboard content column must allow children to shrink inside the viewport",
  );

  assert.match(
    dashboardShell,
    /<main className="min-h-0 min-w-0 flex-1 overflow-y-auto">/,
    "Main dashboard content must scroll internally when a session grows taller than the viewport",
  );
});

test("sidebar keeps footer visible and scrolls its upper content independently", () => {
  const sidebar = read("web/src/components/layout/sidebar.tsx");

  assert.match(
    sidebar,
    /className="flex h-full min-h-0 flex-col bg-sidebar text-sidebar-foreground"/,
    "Sidebar root content must establish a shrinkable flex column",
  );

  assert.match(
    sidebar,
    /className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto[^"]*"/,
    "Sidebar top section must own the scroll when navigation/help content overflows",
  );

  assert.match(
    sidebar,
    /"hidden h-svh overflow-hidden flex-col bg-sidebar text-sidebar-foreground transition-\[width\] duration-200 md:flex"/,
    "Desktop sidebar wrapper must be constrained to the viewport and hide document overflow",
  );
});
