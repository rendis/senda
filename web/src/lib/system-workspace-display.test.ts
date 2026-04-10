import test from "node:test";
import assert from "node:assert/strict";

const {
  SYSTEM_WORKSPACE_LABEL,
  getWorkspaceDisplayName,
  getWorkspaceDisplayCode,
  isSystemWorkspaceLike,
} = await import(new URL("./system-workspace-display.ts", import.meta.url).href);

test("system workspace helpers map technical code to presentation label", () => {
  assert.equal(SYSTEM_WORKSPACE_LABEL, "System");
  assert.equal(isSystemWorkspaceLike({ code: "_system" }), true);
  assert.equal(getWorkspaceDisplayName({ code: "_system", name: "_system" }), "System");
  assert.equal(getWorkspaceDisplayCode({ code: "_system" }), "System");
  assert.equal(getWorkspaceDisplayName({ code: "marketing", name: "Marketing" }), "Marketing");
  assert.equal(getWorkspaceDisplayCode({ code: "marketing" }), "marketing");
});
