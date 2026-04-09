import assert from "node:assert/strict";
import test from "node:test";

const rewritesModule = await import(
  new URL("./src/config/rewrites.ts", import.meta.url).href
);
const { buildRewrites } = rewritesModule;

test("next config only proxies API routes through rewrites", () => {
  assert.deepEqual(buildRewrites("https://api.example.com"), [
    {
      source: "/api/v1/:path*",
      destination: "https://api.example.com/api/v1/:path*",
    },
  ]);
});
