import test from "node:test";
import assert from "node:assert/strict";

const {
  sanitizeWorkspaceCodeInput,
  normalizeWorkspaceCodeInput,
} = await import(new URL("./workspace-code-input.ts", import.meta.url).href);

test("sanitizeWorkspaceCodeInput keeps only lowercase letters, digits, underscores, and hyphens", () => {
  assert.equal(
    sanitizeWorkspaceCodeInput(" Student Portal!__2026 "),
    "studentportal__2026",
  );
});

test("sanitizeWorkspaceCodeInput removes leading separators immediately", () => {
  assert.equal(sanitizeWorkspaceCodeInput("_-main_workspace"), "main_workspace");
});

test("normalizeWorkspaceCodeInput trims trailing separators on blur", () => {
  assert.equal(normalizeWorkspaceCodeInput("main_workspace-_"), "main_workspace");
});
