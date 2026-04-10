import test from "node:test";
import assert from "node:assert/strict";

const { parseTemplateVersionMutationResponse } = await import(
  new URL("./template-version-response.ts", import.meta.url).href,
);

test("publish mutation tolerates 204 no content responses", async () => {
  const response = new Response(null, { status: 204 });

  const result = await parseTemplateVersionMutationResponse(response);

  assert.equal(result, undefined);
});

test("template version mutation still parses json payloads when present", async () => {
  const payload = { id: "ver_123", status: "published" };
  const response = new Response(JSON.stringify(payload), {
    status: 200,
    headers: { "content-type": "application/json" },
  });

  const result = await parseTemplateVersionMutationResponse(response);

  assert.deepEqual(result, payload);
});
