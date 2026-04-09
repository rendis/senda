const defaultBackendOrigin =
  process.env.SENDA_SERVER_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080";

type FetchLike = typeof fetch;

export function buildUpstreamVideoThumbnailUrl(
  requestUrl: URL,
  backendOrigin = defaultBackendOrigin,
): URL {
  const upstream = new URL("/public/video-thumbnail", backendOrigin);
  const rawThumbnailUrl = requestUrl.searchParams.get("url");

  if (rawThumbnailUrl) {
    upstream.searchParams.set("url", rawThumbnailUrl);
  }

  return upstream;
}

function copyProxyHeaders(upstream: Response): Headers {
  const headers = new Headers();
  for (const name of [
    "content-type",
    "cache-control",
    "etag",
    "last-modified",
    "content-length",
  ]) {
    const value = upstream.headers.get(name);
    if (value) {
      headers.set(name, value);
    }
  }
  return headers;
}

export async function proxyVideoThumbnail(
  request: Request,
  backendOrigin = defaultBackendOrigin,
  fetchImpl: FetchLike = fetch,
): Promise<Response> {
  const upstreamUrl = buildUpstreamVideoThumbnailUrl(
    new URL(request.url),
    backendOrigin,
  );

  try {
    const upstream = await fetchImpl(upstreamUrl, {
      method: "GET",
      headers: {
        accept: request.headers.get("accept") || "*/*",
      },
      cache: "no-store",
    });

    return new Response(upstream.body, {
      status: upstream.status,
      headers: copyProxyHeaders(upstream),
    });
  } catch {
    return new Response("video thumbnail proxy failed", {
      status: 502,
      headers: {
        "content-type": "text/plain; charset=utf-8",
      },
    });
  }
}
