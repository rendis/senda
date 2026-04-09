import assert from "node:assert/strict";
import test from "node:test";

const proxyModule = await import(new URL("./proxy.ts", import.meta.url).href);
const { buildUpstreamVideoThumbnailUrl, proxyVideoThumbnail } = proxyModule;

test("buildUpstreamVideoThumbnailUrl targets the backend thumbnail endpoint", () => {
  const upstream = buildUpstreamVideoThumbnailUrl(
    new URL(
      "https://senda.tether.education/public/video-thumbnail?url=https://img.youtube.com/vi/-pAu40lToXQ/maxresdefault.jpg",
    ),
    "https://backend.example.com",
  );

  assert.equal(
    upstream.toString(),
    "https://backend.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2F-pAu40lToXQ%2Fmaxresdefault.jpg",
  );
});

test("proxyVideoThumbnail streams the backend image response", async () => {
  let requestedUrl = "";
  const body = Uint8Array.from([1, 2, 3, 4]);

  const response = await proxyVideoThumbnail(
    new Request(
      "https://senda.tether.education/public/video-thumbnail?url=https://img.youtube.com/vi/-pAu40lToXQ/maxresdefault.jpg",
    ),
    "https://backend.example.com",
    async (input: string | URL | Request) => {
      requestedUrl = String(input);
      return new Response(body, {
        status: 200,
        headers: {
          "content-type": "image/png",
          "cache-control": "public, max-age=60",
        },
      });
    },
  );

  assert.equal(
    requestedUrl,
    "https://backend.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2F-pAu40lToXQ%2Fmaxresdefault.jpg",
  );
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("content-type"), "image/png");
  assert.equal(response.headers.get("cache-control"), "public, max-age=60");
  assert.deepEqual(new Uint8Array(await response.arrayBuffer()), body);
});
