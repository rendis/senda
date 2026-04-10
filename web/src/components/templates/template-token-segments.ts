export type TokenCategory = "event" | "injector";

export type TemplateTokenSegment =
  | {
      kind: "text";
      id: string;
      text: string;
    }
  | {
      kind: "token";
      id: string;
      token: string;
      label: string;
      category: TokenCategory;
    };

const EVENT_TOKEN_CHIP_CLASSNAME =
  "inline-flex max-w-full items-center rounded border border-dashed border-input bg-muted px-1.5 py-0.5 text-xs align-middle select-none truncate text-foreground";

const INJECTOR_TOKEN_CHIP_CLASSNAME =
  "inline-flex max-w-full items-center rounded border border-dashed border-violet-400 bg-violet-50 px-1.5 py-0.5 text-xs align-middle select-none truncate text-violet-700 dark:border-violet-600 dark:bg-violet-950 dark:text-violet-300";

function nowId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export function normalizeVariableToken(raw: string) {
  return raw
    .replace(/^\s*\{\{\s*/, "")
    .replace(/\s*\}\}\s*$/, "")
    .replace(/\s+/g, " ")
    .trim();
}

export function variableToPlaceholder(rawToken: string) {
  return `{{ ${normalizeVariableToken(rawToken)} }}`;
}

export function createTextSegment(raw: string): TemplateTokenSegment {
  return {
    kind: "text",
    id: nowId(),
    text: raw,
  };
}

export function createTokenSegment(
  token: string,
  category: TokenCategory,
  label?: string,
): TemplateTokenSegment {
  const normalizedToken = normalizeVariableToken(token);
  return {
    kind: "token",
    id: nowId(),
    token: normalizedToken,
    label: label ?? normalizedToken,
    category,
  };
}

export function guessSegmentCategory(rawToken: string): TokenCategory {
  const token = normalizeVariableToken(rawToken);
  if (token.startsWith("injector.")) return "injector";
  return "event";
}

export function mergeAdjacentTextSegments(
  segments: TemplateTokenSegment[],
): TemplateTokenSegment[] {
  const merged: TemplateTokenSegment[] = [];

  for (const segment of segments) {
    if (segment.kind === "text") {
      const last = merged[merged.length - 1];
      if (last?.kind === "text") {
        last.text += segment.text;
        continue;
      }
      merged.push({ ...segment });
      continue;
    }
    merged.push(segment);
  }

  const compact = merged.filter(
    (segment) => segment.kind !== "text" || segment.text.length > 0,
  );
  const hasText = compact.some((segment) => segment.kind === "text");

  if (!compact.length) {
    return [createTextSegment("")];
  }
  if (!hasText) {
    return [...compact, createTextSegment("")];
  }

  return compact;
}

export function ensureUniqueSegmentIds(
  segments: TemplateTokenSegment[],
): TemplateTokenSegment[] {
  const seen = new Set<string>();
  return segments.map((segment) => {
    if (!segment.id || seen.has(segment.id)) {
      const nextId = nowId();
      seen.add(nextId);
      return {
        ...segment,
        id: nextId,
      };
    }
    seen.add(segment.id);
    return segment;
  });
}

export function parseTextChunkToSegments(raw: string) {
  if (!raw) return [];

  const text = raw.replace(/\u200b/g, "");
  if (!text) return [];

  const parts = text.split(/(\{\{[^}]+\}\})/g);
  const segments: TemplateTokenSegment[] = [];

  for (const part of parts) {
    if (!part) continue;
    if (/^\{\{[^}]+\}\}$/.test(part.trim())) {
      const normalized = normalizeVariableToken(part);
      if (normalized) {
        segments.push(
          createTokenSegment(normalized, guessSegmentCategory(normalized), normalized),
        );
      }
      continue;
    }
    segments.push(createTextSegment(part));
  }

  return segments;
}

export function parseContentToSegments(content: string): TemplateTokenSegment[] {
  const raw = typeof content === "string" ? content : "";
  if (!raw) {
    return [createTextSegment("")];
  }

  const segments = parseTextChunkToSegments(raw);
  if (!segments.length) {
    return [createTextSegment("")];
  }
  return ensureUniqueSegmentIds(mergeAdjacentTextSegments(segments));
}

export function renderSegmentsToText(segments: TemplateTokenSegment[]) {
  return segments
    .map((segment) =>
      segment.kind === "text" ? segment.text : variableToPlaceholder(segment.token),
    )
    .join("");
}

export function getTokenChipText(segment: Pick<Extract<TemplateTokenSegment, { kind: "token" }>, "label" | "token">) {
  const label = segment.label?.trim();
  return label && label.length > 0 ? label : normalizeVariableToken(segment.token);
}

export function getTokenChipClassName(category: TokenCategory) {
  return category === "injector"
    ? INJECTOR_TOKEN_CHIP_CLASSNAME
    : EVENT_TOKEN_CHIP_CLASSNAME;
}
