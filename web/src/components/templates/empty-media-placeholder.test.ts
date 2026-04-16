import test from "node:test";
import assert from "node:assert/strict";

const { emptyMediaPlaceholder, resolveSrcForMjml } = await import(
  new URL("./empty-media-placeholder.ts", import.meta.url).href
);

const DATA_URI_PREFIX = "data:image/svg+xml,";

test("emptyMediaPlaceholder starts with data:image/svg+xml", () => {
  const result = emptyMediaPlaceholder("image");
  assert.ok(result.startsWith(DATA_URI_PREFIX), `Expected data URI, got: ${result.slice(0, 40)}`);
});

function decodePlaceholder(result: string): string {
  return decodeURIComponent(result.replace(DATA_URI_PREFIX, ""));
}

test("emptyMediaPlaceholder image defaults to 600x200", () => {
  const result = emptyMediaPlaceholder("image");
  const decoded = decodePlaceholder(result);
  assert.ok(decoded.includes('width="600"'), `Missing width 600`);
  assert.ok(decoded.includes('height="200"'), `Missing height 200`);
});

test("emptyMediaPlaceholder video defaults to 600x340", () => {
  const result = emptyMediaPlaceholder("video");
  const decoded = decodePlaceholder(result);
  assert.ok(decoded.includes('width="600"'), `Missing width 600`);
  assert.ok(decoded.includes('height="340"'), `Missing height 340`);
});

test("emptyMediaPlaceholder respects explicit dimensions", () => {
  const result = emptyMediaPlaceholder("image", { width: 400, height: 150 });
  const decoded = decodePlaceholder(result);
  assert.ok(decoded.includes('width="400"'), `Missing width 400`);
  assert.ok(decoded.includes('height="150"'), `Missing height 150`);
});

test("emptyMediaPlaceholder is URL-encoded not base64", () => {
  const result = emptyMediaPlaceholder("image");
  assert.ok(!result.startsWith("data:image/svg+xml;base64"), "Must not be base64");
  assert.ok(result.startsWith("data:image/svg+xml,"), "Must use URL encoding (comma separator)");
});

test("emptyMediaPlaceholder is a pure function (deterministic)", () => {
  const a = emptyMediaPlaceholder("image", { width: 600, height: 200 });
  const b = emptyMediaPlaceholder("image", { width: 600, height: 200 });
  assert.equal(a, b, "Same inputs must produce same output");
});

// resolveSrcForMjml tests

test("resolveSrcForMjml token passthrough: single token", () => {
  const token = "{{ injector.user.avatar }}";
  assert.equal(resolveSrcForMjml(token, "image"), token);
});

test("resolveSrcForMjml token passthrough: hybrid string (URL + token)", () => {
  const hybrid = "https://cdn.example.com/{{ injector.user.id }}/photo.jpg";
  assert.equal(resolveSrcForMjml(hybrid, "image"), hybrid);
});

test("resolveSrcForMjml empty string returns placeholder", () => {
  const result = resolveSrcForMjml("", "image");
  assert.ok(result.startsWith("data:image/svg+xml"), "Empty → placeholder");
});

test("resolveSrcForMjml whitespace-only returns placeholder", () => {
  const result = resolveSrcForMjml("   ", "image");
  assert.ok(result.startsWith("data:image/svg+xml"), "Whitespace → placeholder");
});

test("resolveSrcForMjml valid URL is returned directly", () => {
  const url = "https://example.com/img.png";
  assert.equal(resolveSrcForMjml(url, "image"), url);
});

test("resolveSrcForMjml never calls placeholder when token present", () => {
  const token = "{{ injector.user.avatar }}";
  const result = resolveSrcForMjml(token, "image");
  assert.ok(!result.startsWith("data:image/svg+xml"), "Token → no placeholder");
});
