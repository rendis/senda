const DEFAULT_IMAGE_WIDTH = 600;
const DEFAULT_IMAGE_HEIGHT = 200;
const DEFAULT_VIDEO_WIDTH = 600;
const DEFAULT_VIDEO_HEIGHT = 340;

const TOKEN_PATTERN = /\{\{[^}]+\}\}/;

const DEFAULT_IMAGE_PLACEHOLDER = buildPlaceholderUri("No image", DEFAULT_IMAGE_WIDTH, DEFAULT_IMAGE_HEIGHT);
const DEFAULT_VIDEO_PLACEHOLDER = buildPlaceholderUri("No video", DEFAULT_VIDEO_WIDTH, DEFAULT_VIDEO_HEIGHT);

function parseFixedPixelWidth(widthStr?: string): number | undefined {
  if (!widthStr) return undefined;
  const trimmed = widthStr.trim();
  if (!/^\d+(?:px)?$/i.test(trimmed)) return undefined;
  return Number.parseInt(trimmed, 10) || undefined;
}

function buildPlaceholderUri(label: string, width: number, height: number): string {
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">` +
    `<rect width="100%" height="100%" fill="#e5e7eb"/>` +
    `<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" font-family="sans-serif" font-size="14" fill="#9ca3af">${label}</text>` +
    `</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

export function emptyMediaPlaceholder(
  kind: "image" | "video",
  opts?: { width?: number; height?: number },
): string {
  if (!opts?.width && !opts?.height) {
    return kind === "video" ? DEFAULT_VIDEO_PLACEHOLDER : DEFAULT_IMAGE_PLACEHOLDER;
  }
  const width = opts.width ?? (kind === "video" ? DEFAULT_VIDEO_WIDTH : DEFAULT_IMAGE_WIDTH);
  const height = opts.height ?? (kind === "video" ? DEFAULT_VIDEO_HEIGHT : DEFAULT_IMAGE_HEIGHT);
  const label = kind === "video" ? "No video" : "No image";
  return buildPlaceholderUri(label, width, height);
}

/**
 * Resolves the `src` attribute value for MJML serialization.
 *
 * Precedence:
 * 1. If the raw value contains a `{{ ... }}` token → return as-is (token passthrough).
 * 2. If the raw value is empty or whitespace-only → return placeholder data URI.
 * 3. Otherwise → return the raw value directly (URL or other non-empty string).
 */
export function resolveSrcForMjml(
  raw: string,
  kind: "image" | "video",
  widthStr?: string,
): string {
  if (TOKEN_PATTERN.test(raw)) {
    return raw;
  }
  if (!raw.trim()) {
    const width = parseFixedPixelWidth(widthStr);
    return emptyMediaPlaceholder(kind, width ? { width } : undefined);
  }
  return raw;
}
