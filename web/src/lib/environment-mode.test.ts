import assert from "node:assert/strict";
import test from "node:test";

import {
  applyEnvironmentSearchParam,
  normalizeEnvironment,
  resolveEnvironmentSwitchPath,
} from "./environment-mode.ts";

test("normalizeEnvironment defaults invalid values to prod", () => {
  assert.equal(normalizeEnvironment("prod"), "prod");
  assert.equal(normalizeEnvironment("test"), "test");
  assert.equal(normalizeEnvironment(""), "prod");
  assert.equal(normalizeEnvironment("dev"), "prod");
  assert.equal(normalizeEnvironment(undefined), "prod");
});

test("applyEnvironmentSearchParam preserves query params and hash", () => {
  assert.equal(
    applyEnvironmentSearchParam("/t/acme/w/marketing/templates?tab=content#preview", "test"),
    "/t/acme/w/marketing/templates?tab=content&environment=test#preview",
  );
});

test("applyEnvironmentSearchParam overwrites the previous environment value", () => {
  assert.equal(
    applyEnvironmentSearchParam("/t/acme/w/marketing?environment=prod", "test"),
    "/t/acme/w/marketing?environment=test",
  );
});

test("resolveEnvironmentSwitchPath returns workspace dashboard on environment change", () => {
  assert.equal(
    resolveEnvironmentSwitchPath(
      "/t/acme/w/marketing/templates/welcome/edit?templateId=123&versionId=456&environment=prod#preview",
      "test",
    ),
    "/t/acme/w/marketing?environment=test",
  );
});
