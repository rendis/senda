export type VideoBlockLike = {
  videoUrl: string;
  thumbnailUrl: string;
  alt: string;
  width: string;
  align: "left" | "center" | "right";
};

export function extractVideoThumbnail(url: string): string {
  if (!url) return "";
  const ytMatch = url.match(
    /(?:youtube\.com\/(?:watch\?.*v=|embed\/)|youtu\.be\/)([a-zA-Z0-9_-]{11})/,
  );
  if (ytMatch?.[1]) {
    return `https://img.youtube.com/vi/${ytMatch[1]}/maxresdefault.jpg`;
  }

  const vimeoMatch = url.match(/vimeo\.com\/(\d+)/);
  if (vimeoMatch?.[1]) {
    return `https://vumbnail.com/${vimeoMatch[1]}.jpg`;
  }

  return "";
}

const VIDEO_THUMBNAIL_PATH = "/public/video-thumbnail";

export function extractOriginalThumbnailUrl(src: string): string {
  if (!src) return "";

  try {
    const parsed = new URL(src);
    if (parsed.pathname === VIDEO_THUMBNAIL_PATH) {
      return parsed.searchParams.get("url") || src;
    }
  } catch {
    if (src.includes(VIDEO_THUMBNAIL_PATH + "?url=")) {
      const idx = src.indexOf("?url=");
      return decodeURIComponent(src.slice(idx + 5));
    }
  }

  return src;
}

export function renderVideoBlockToMjml(block: VideoBlockLike): string {
  const href = block.videoUrl ? ` href="${block.videoUrl}"` : "";
  const width = block.width ? ` width="${block.width}"` : "";
  const alt = block.alt ? ` alt="${block.alt}"` : "";
  return `\n<mj-image src="${block.thumbnailUrl || ""}"${href}${width}${alt} align="${block.align}" css-class="senda-video" />`;
}
