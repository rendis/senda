export function buildRewrites(
  apiOrigin: string,
  videoThumbnailOrigin = apiOrigin,
) {
  return [
    {
      source: "/api/v1/:path*",
      destination: `${apiOrigin}/api/v1/:path*`,
    },
    {
      source: "/public/video-thumbnail",
      destination: `${videoThumbnailOrigin}/public/video-thumbnail`,
    },
  ];
}
