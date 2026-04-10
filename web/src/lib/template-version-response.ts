export async function parseTemplateVersionMutationResponse<T>(
  response: Response,
): Promise<T | undefined> {
  if (response.status === 204) {
    return undefined;
  }

  const contentLength = response.headers.get("content-length");
  if (contentLength === "0") {
    return undefined;
  }

  const rawBody = await response.text();
  if (!rawBody.trim()) {
    return undefined;
  }

  return JSON.parse(rawBody) as T;
}
