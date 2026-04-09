export function buildRewrites(apiOrigin: string) {
  return [
    {
      source: "/api/v1/:path*",
      destination: `${apiOrigin}/api/v1/:path*`,
    },
  ];
}
