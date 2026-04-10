import {
  guessSegmentCategory,
  normalizeVariableToken,
  type TokenCategory,
} from "./template-token-segments.ts";

export type PreviewTokenMeta = {
  label?: string;
  category?: TokenCategory;
  static?: boolean;
  source?: "database" | "code";
};

export type PreviewTextSegment =
  | {
      kind: "text";
      text: string;
    }
  | {
      kind: "token";
      token: string;
      label: string;
      title: string;
      category: TokenCategory;
      static?: boolean;
      source?: "database" | "code";
    };

const PREVIEW_PLACEHOLDER_PATTERN = /\{\{\s*([^}]+?)\s*\}\}/g;
const PREVIEW_TOKEN_STYLE_ID = "senda-preview-token-badges";
const TEXT_NODE_FILTER = 4;
const PREVIEW_TOKEN_CLASSNAME = "senda-preview-token";
const PREVIEW_TOKEN_INJECTOR_CLASSNAME = "senda-preview-token--injector";
const PREVIEW_TOKEN_EVENT_CLASSNAME = "senda-preview-token--event";

const PREVIEW_TOKEN_STYLES = `
.${PREVIEW_TOKEN_CLASSNAME} {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  border-radius: 999px;
  border: 1px dashed #cbd5e1;
  background: #f8fafc;
  color: #0f172a;
  padding: 0.05rem 0.4rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 0.85em;
  font-weight: 500;
  line-height: 1.35;
  vertical-align: baseline;
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.${PREVIEW_TOKEN_INJECTOR_CLASSNAME} {
  border-color: #a78bfa;
  background: #f5f3ff;
  color: #6d28d9;
}

.${PREVIEW_TOKEN_EVENT_CLASSNAME} {
  border-color: #cbd5e1;
  background: #f8fafc;
  color: #334155;
}
`;

function formatPreviewTokenTitle(token: string, meta?: PreviewTokenMeta) {
  const parts = [normalizeVariableToken(token)];
  if (meta?.static) {
    parts.push("static default");
  }
  if (meta?.source === "code") {
    parts.push("code injector");
  }
  return parts.join(" · ");
}

export function formatPreviewTokenLabel(token: string, meta?: PreviewTokenMeta) {
  const candidate = meta?.label?.trim();
  if (candidate) {
    return candidate;
  }

  const normalized = normalizeVariableToken(token);
  if (normalized.startsWith("injector.")) {
    return normalized.slice("injector.".length);
  }
  if (normalized.startsWith("event.")) {
    return normalized.slice("event.".length);
  }
  return normalized;
}

export function parsePreviewTextSegments(
  rawText: string,
  resolveMeta?: (token: string) => PreviewTokenMeta | undefined,
): PreviewTextSegment[] {
  if (!rawText) {
    return [];
  }

  const parts = rawText.split(/(\{\{\s*[^}]+?\s*\}\})/g);
  const segments: PreviewTextSegment[] = [];

  for (const part of parts) {
    if (!part) {
      continue;
    }

    if (/^\{\{\s*[^}]+?\s*\}\}$/.test(part.trim())) {
      const token = normalizeVariableToken(part);
      if (!token) {
        continue;
      }
      const meta = resolveMeta?.(token);
      segments.push({
        kind: "token",
        token,
        label: formatPreviewTokenLabel(token, meta),
        title: formatPreviewTokenTitle(token, meta),
        category: meta?.category ?? guessSegmentCategory(token),
        static: meta?.static,
        source: meta?.source,
      });
      continue;
    }

    segments.push({
      kind: "text",
      text: part,
    });
  }

  return segments;
}

function hasPreviewPlaceholderTokens(rawText: string) {
  PREVIEW_PLACEHOLDER_PATTERN.lastIndex = 0;
  return PREVIEW_PLACEHOLDER_PATTERN.test(rawText);
}

function shouldDecorateTextNode(node: Text) {
  const parent = node.parentElement;
  if (!parent) {
    return false;
  }

  if (parent.closest("[data-senda-preview-token]")) {
    return false;
  }

  const tagName = parent.tagName;
  if (tagName === "SCRIPT" || tagName === "STYLE" || tagName === "NOSCRIPT" || tagName === "TEXTAREA") {
    return false;
  }

  return hasPreviewPlaceholderTokens(node.textContent ?? "");
}

function ensurePreviewTokenStyles(doc: Document) {
  if (doc.getElementById(PREVIEW_TOKEN_STYLE_ID)) {
    return;
  }

  const style = doc.createElement("style");
  style.id = PREVIEW_TOKEN_STYLE_ID;
  style.textContent = PREVIEW_TOKEN_STYLES;
  doc.head?.appendChild(style);
}

export function decoratePreviewDocumentPlaceholders(
  doc: Document,
  resolveMeta?: (token: string) => PreviewTokenMeta | undefined,
) {
  if (!doc.body) {
    return false;
  }

  ensurePreviewTokenStyles(doc);

  const view = doc.defaultView;
  const filter = view?.NodeFilter;
  const walker = doc.createTreeWalker(
    doc.body,
    filter?.SHOW_TEXT ?? TEXT_NODE_FILTER,
    {
      acceptNode(node) {
        return shouldDecorateTextNode(node as Text)
          ? filter?.FILTER_ACCEPT ?? 1
          : filter?.FILTER_REJECT ?? 2;
      },
    },
  );

  const textNodes: Text[] = [];
  let currentNode = walker.nextNode();
  while (currentNode) {
    textNodes.push(currentNode as Text);
    currentNode = walker.nextNode();
  }

  if (textNodes.length === 0) {
    return false;
  }

  for (const textNode of textNodes) {
    const segments = parsePreviewTextSegments(textNode.textContent ?? "", resolveMeta);
    if (!segments.some((segment) => segment.kind === "token")) {
      continue;
    }

    const fragment = doc.createDocumentFragment();
    for (const segment of segments) {
      if (segment.kind === "text") {
        fragment.appendChild(doc.createTextNode(segment.text));
        continue;
      }

      const chip = doc.createElement("span");
      chip.setAttribute("data-senda-preview-token", "true");
      chip.className = [
        PREVIEW_TOKEN_CLASSNAME,
        segment.category === "injector"
          ? PREVIEW_TOKEN_INJECTOR_CLASSNAME
          : PREVIEW_TOKEN_EVENT_CLASSNAME,
      ].join(" ");
      chip.textContent = segment.label;
      chip.title = segment.title;
      chip.setAttribute("aria-label", segment.title);
      if (segment.static) {
        chip.dataset.static = "true";
      }
      if (segment.source) {
        chip.dataset.source = segment.source;
      }
      fragment.appendChild(chip);
    }

    textNode.parentNode?.replaceChild(fragment, textNode);
  }

  return true;
}
