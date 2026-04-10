import test from "node:test";
import assert from "node:assert/strict";

const {
  formatPreviewTokenLabel,
  parsePreviewTextSegments,
} = await import(new URL("./preview-token-badges.ts", import.meta.url).href);

const USER_NAME_TOKEN = "event.user_name";

test("formatPreviewTokenLabel removes moustache-oriented prefixes for unknown tokens", () => {
  assert.equal(
    formatPreviewTokenLabel("injector.request_debug.workspace_code"),
    "request_debug.workspace_code",
  );
  assert.equal(
    formatPreviewTokenLabel(USER_NAME_TOKEN),
    "user_name",
  );
});

test("parsePreviewTextSegments converts unresolved placeholders into readable badge segments", () => {
  const segments = parsePreviewTextSegments(
    "WORKSPACE={{ injector.request_debug.workspace_code }} | EVENT={{ event.user_name }}",
    (token: string) => {
      if (token === USER_NAME_TOKEN) {
        return {
          label: "user_name",
          category: "event",
        };
      }
      return undefined;
    },
  );

  assert.deepEqual(segments, [
    { kind: "text", text: "WORKSPACE=" },
    {
      kind: "token",
      token: "injector.request_debug.workspace_code",
      label: "request_debug.workspace_code",
      title: "injector.request_debug.workspace_code",
      category: "injector",
      static: undefined,
      source: undefined,
    },
    { kind: "text", text: " | EVENT=" },
    {
      kind: "token",
      token: USER_NAME_TOKEN,
      label: "user_name",
      title: USER_NAME_TOKEN,
      category: "event",
      static: undefined,
      source: undefined,
    },
  ]);
});

test("parsePreviewTextSegments preserves static/code metadata for known placeholders", () => {
  const segments = parsePreviewTextSegments(
    "{{ injector.workspace_profile.workspace_code }}",
    () => ({
      label: "workspace_profile.workspace_code",
      category: "injector",
      static: true,
      source: "code",
    }),
  );

  assert.deepEqual(segments, [
    {
      kind: "token",
      token: "injector.workspace_profile.workspace_code",
      label: "workspace_profile.workspace_code",
      title: "injector.workspace_profile.workspace_code · static default · code injector",
      category: "injector",
      static: true,
      source: "code",
    },
  ]);
});
