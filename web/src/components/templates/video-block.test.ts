import test from "node:test";
import assert from "node:assert/strict";

const {
  extractOriginalThumbnailUrl,
  extractVideoThumbnail,
  renderVideoBlockToMjml,
} = await import(new URL("./video-block.ts", import.meta.url).href);

test("renderVideoBlockToMjml preserves the raw thumbnail URL instead of proxying it", () => {
  const mjml = renderVideoBlockToMjml({
    videoUrl: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
    thumbnailUrl: "https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
    alt: "Preview",
    width: "600px",
    align: "center",
  });

  assert.match(
    mjml,
    /src="https:\/\/img\.youtube\.com\/vi\/dQw4w9WgXcQ\/maxresdefault\.jpg"/,
  );
  assert.doesNotMatch(mjml, /public\/video-thumbnail/);
  assert.doesNotMatch(mjml, /localhost:8081/);
});

test("extractOriginalThumbnailUrl unwraps legacy composite thumbnail URLs", () => {
  assert.equal(
    extractOriginalThumbnailUrl(
      "https://api.example.com/public/video-thumbnail?url=https%3A%2F%2Fimg.youtube.com%2Fvi%2FdQw4w9WgXcQ%2Fmaxresdefault.jpg",
    ),
    "https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
  );
});

test("extractVideoThumbnail derives the youtube thumbnail directly from the video URL", () => {
  assert.equal(
    extractVideoThumbnail("https://www.youtube.com/watch?v=dQw4w9WgXcQ"),
    "https://img.youtube.com/vi/dQw4w9WgXcQ/maxresdefault.jpg",
  );
});
