import assert from "node:assert/strict";
import test from "node:test";

const rewritesModule = await import(
  new URL("./src/config/rewrites.ts", import.meta.url).href
);
const { buildRewrites } = rewritesModule;

test("next config proxies public video thumbnails to the backend origin", () => {
  assert.deepEqual(buildRewrites("https://api.example.com"), [
    {
      source: "/api/v1/:path*",
      destination: "https://api.example.com/api/v1/:path*",
    },
    {
      source: "/public/video-thumbnail",
      destination: "https://api.example.com/public/video-thumbnail",
    },
  ]);
});

test("next config can proxy video thumbnails to a dedicated backend origin", () => {
  assert.deepEqual(
    buildRewrites(
      "https://public.example.com",
      "https://backend.example.com",
    ),
    [
      {
        source: "/api/v1/:path*",
        destination: "https://public.example.com/api/v1/:path*",
      },
      {
        source: "/public/video-thumbnail",
        destination: "https://backend.example.com/public/video-thumbnail",
      },
    ],
  );
});
