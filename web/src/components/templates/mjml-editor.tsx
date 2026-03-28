"use client";

import {
  type ComponentType,
  type ClipboardEvent as ReactClipboardEvent,
  type DragEvent,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type SyntheticEvent as ReactSyntheticEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useMinimumLoading } from "@/hooks/use-minimum-loading";
import { useRouter, useSearchParams, useParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTheme } from "next-themes";
import {
  ArrowLeft,
  Save,
  Send,
  Rocket,
  MonitorStop,
  Paintbrush,
  Trash2,
  GripVertical,
  Type,
  Image as ImageIcon,
  Minus,
  Grip,
  Plus,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  ChevronsDownUp,
  ChevronsUpDown,
  Search,
  Play,
  List,
  LayoutTemplate,
  X,
} from "lucide-react";
import { TextBlockEditor, type TextBlockEditorHandle } from "./text-block-editor";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useTemplateVersion,
  useSaveTemplateVersion,
  usePublishVersion,
  usePreviewMjml,
  useTemplateVersionLocales,
  useTemplateLocale,
  useSaveTemplateLocale,
  useDeleteTemplateLocale,
} from "@/hooks/use-template-version";
import { useTemplateType } from "@/hooks/use-template-types";
import { useInjectorList } from "@/hooks/use-injectors";
import { useApi } from "@/hooks/use-api";
import { useAutoSave } from "@/hooks/use-auto-save";
import { SaveStatusIndicator } from "@/components/templates/save-status-indicator";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { TestSendModal } from "@/components/templates/test-send-modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from "@/components/ui/popover";
import { useTranslations } from "next-intl";
import { toast } from "sonner";
import type { InjectorDefinition, InjectorWithValues } from "@/types/injectors";

const metadataSchema = z.object({
  subject: z.string().min(1, { message: "Subject is required" }),
  preview_text: z.string().optional(),
  from_name: z.string().min(1, { message: "From name is required" }),
  reply_to: z.string().optional(),
});

type MetadataForm = z.infer<typeof metadataSchema>;

type BuilderBlockType = "text" | "button" | "image" | "divider" | "spacer" | "banner" | "video" | "list";

type BuilderSegment =
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
      category: "event" | "injector";
    };

type BuilderTextBlock = {
  id: string;
  label?: string;
  type: "text";
  content: string;
  align: "left" | "center" | "right" | "justify";
};

type BuilderButtonBlock = {
  id: string;
  label?: string;
  type: "button";
  segments: BuilderSegment[];
  href: string;
  align: "left" | "center" | "right";
};

type BuilderImageBlock = {
  id: string;
  label?: string;
  type: "image";
  src: string;
  alt?: string;
  width?: string;
  align: "left" | "center" | "right";
};

type BuilderDividerBlock = {
  id: string;
  label?: string;
  type: "divider";
};

type BuilderSpacerBlock = {
  id: string;
  label?: string;
  type: "spacer";
  height: number;
};

type BuilderBannerBlock = {
  id: string;
  label?: string;
  type: "banner";
  backgroundUrl: string;
  backgroundColor: string;
  mode: "fixed-height" | "fluid-height";
  height: number;
  segments: BuilderSegment[];
  buttonText: string;
  buttonHref: string;
  buttonColor: string;
  verticalAlign: "top" | "middle" | "bottom";
  align: "left" | "center" | "right";
  padding: number;
};

type BuilderVideoBlock = {
  id: string;
  label?: string;
  type: "video";
  videoUrl: string;
  thumbnailUrl: string;
  alt: string;
  width: string;
  align: "left" | "center" | "right";
};

type ListItem = {
  id: string;
  segments: BuilderSegment[];
  children: ListItem[];
};

type BuilderListBlock = {
  id: string;
  label?: string;
  type: "list";
  listType: "bullet" | "number" | "letter-upper" | "letter-lower" | "roman";
  items: ListItem[];
  align: "left" | "center" | "right";
};

type BuilderBlock =
  | BuilderTextBlock
  | BuilderButtonBlock
  | BuilderImageBlock
  | BuilderDividerBlock
  | BuilderSpacerBlock
  | BuilderBannerBlock
  | BuilderVideoBlock
  | BuilderListBlock;

type BuilderDocument = {
  version: number;
  blocks: BuilderBlock[];
};

// defaultBlockLabel is now built inside MjmlEditor using t() for i18n

type EditorMode = "visual" | "code";

type TemplateVariable = {
  id: string;
  token: string;
  label: string;
  hint: string;
  category: "event" | "injector";
};

type PreviewStageSize = { width: number; height: number };
type PreviewDocumentSize = { width: number; height: number };
type PreviewSplitMode = "ratio" | "px";

const DEFAULT_BLOCK_WIDTH = "w-64";
const MIN_PANEL_WIDTH = 280;
const PANEL_RESIZER_WIDTH = 12;
const DEFAULT_PREVIEW_DOCUMENT_SIZE: PreviewDocumentSize = {
  width: 900,
  height: 700,
};
const VARIABLE_DND_MIME = "application/x-senda-variable";
const BLOCK_DND_MIME = "application/x-senda-block-id";
const LIST_ITEM_DND_MIME = "application/x-senda-list-item";
const CLIPBOARD_SEGMENTS_MIME = "application/x-senda-segments";
const TOKEN_SEGMENT_KIND = "token";
const TOKEN_CHIP_CLASSNAME =
  "inline-flex items-center rounded border border-dashed border-input bg-muted px-1.5 py-0.5 text-xs align-middle select-none";
const MIN_PREVIEW_SCALE = 0.01;

function nowId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return !!v && typeof v === "object" && !Array.isArray(v);
}

function createTextSegment(raw: string) {
  return {
    kind: "text" as const,
    id: nowId(),
    text: raw,
  };
}

function createTokenSegment(
  token: string,
  category: "event" | "injector",
  label?: string
) {
  return {
    kind: "token" as const,
    id: nowId(),
    token: normalizeVariableToken(token),
    label: label ?? normalizeVariableToken(token),
    category,
  };
}

function guessSegmentCategory(rawToken: string): "event" | "injector" {
  const token = normalizeVariableToken(rawToken);
  if (token.startsWith("event.")) return "event";
  if (token.startsWith("injector.")) return "injector";
  return "event";
}

function parseContentToSegments(content: string): BuilderSegment[] {
  const raw = typeof content === "string" ? content : "";
  if (!raw) {
    return [createTextSegment("")];
  }

  const parts = raw.split(/(\{\{[^}]+\}\})/g);
  const segments: BuilderSegment[] = [];

  for (const part of parts) {
    if (!part) {
      continue;
    }

    if (/^\{\{[^}]+\}\}$/.test(part.trim())) {
      const normalized = normalizeVariableToken(part);
      if (normalized) {
        segments.push(
          createTokenSegment(normalized, guessSegmentCategory(normalized), normalized)
        );
      }
      continue;
    }

    segments.push({
      kind: "text",
      id: nowId(),
      text: part,
    });
  }

  if (!segments.length) {
    return [createTextSegment("")];
  }

  return segments;
}

function parseTextChunkToSegments(raw: string) {
  if (!raw) return [];

  const text = raw.replace(/\u200b/g, "");
  if (!text) return [];

  const parts = text.split(/(\{\{[^}]+\}\})/g);
  const segments: BuilderSegment[] = [];

  for (const part of parts) {
    if (!part) continue;
    if (/^\{\{[^}]+\}\}$/.test(part.trim())) {
      const normalized = normalizeVariableToken(part);
      if (normalized) {
        segments.push(
          createTokenSegment(normalized, guessSegmentCategory(normalized), normalized)
        );
      }
      continue;
    }
    segments.push(createTextSegment(part));
  }

  return segments;
}

function isTokenChipNode(node: Node): node is HTMLElement {
  return (
    node.nodeType === Node.ELEMENT_NODE &&
    (node as HTMLElement).dataset.segmentKind === TOKEN_SEGMENT_KIND
  );
}

function parseSegmentsFromEditorNode(root: HTMLElement) {
  const segments: BuilderSegment[] = [];

  function collect(nodes: NodeListOf<ChildNode> | ChildNode[]) {
    for (const node of nodes) {
      if (node.nodeType === Node.TEXT_NODE) {
        segments.push(...parseTextChunkToSegments(node.textContent ?? ""));
        continue;
      }

      if (node.nodeType !== Node.ELEMENT_NODE) {
        continue;
      }

      const element = node as HTMLElement;
      if (element.dataset.segmentKind === TOKEN_SEGMENT_KIND) {
        const token = normalizeVariableToken(
          element.dataset.token ?? element.textContent ?? ""
        );
        if (!token) {
          continue;
        }
        segments.push({
          kind: "token",
          id: element.dataset.segmentId?.trim() || nowId(),
          token,
          label: element.dataset.label?.trim() || token,
          category: element.dataset.category === "injector" ? "injector" : "event",
        });
        continue;
      }

      if (element.tagName === "BR") {
        segments.push(createTextSegment("\n"));
        continue;
      }

      collect(element.childNodes);
      if (element.tagName === "DIV" || element.tagName === "P") {
        segments.push(createTextSegment("\n"));
      }
    }
  }

  collect(root.childNodes);
  return ensureUniqueSegmentIds(mergeAdjacentTextSegments(segments));
}

function segmentsMeaningfullyEqual(a: BuilderSegment[], b: BuilderSegment[]) {
  if (a.length !== b.length) return false;
  for (let index = 0; index < a.length; index += 1) {
    const left = a[index];
    const right = b[index];
    if (!left || !right || left.kind !== right.kind) return false;
    if (left.kind === "text" && right.kind === "text" && left.text !== right.text) {
      return false;
    }
    if (left.kind === "token" && right.kind === "token") {
      if (
        left.token !== right.token ||
        left.label !== right.label ||
        left.category !== right.category
      ) {
        return false;
      }
    }
  }
  return true;
}

function ensureUniqueSegmentIds(segments: BuilderSegment[]) {
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

function countSegmentUnits(segment: BuilderSegment) {
  if (segment.kind === "token") return 1;
  return segment.text.length;
}

function countSegmentsUnits(segments: BuilderSegment[]) {
  return segments.reduce((total, segment) => total + countSegmentUnits(segment), 0);
}

function countUnitsInDomNode(node: Node): number {
  if (node.nodeType === Node.TEXT_NODE) {
    return node.textContent?.length ?? 0;
  }

  if (isTokenChipNode(node)) {
    return 1;
  }

  let total = 0;
  node.childNodes.forEach((child) => {
    total += countUnitsInDomNode(child);
  });
  return total;
}

function getSelectionInsideEditor(editor: HTMLElement) {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) {
    return null;
  }
  const range = selection.getRangeAt(0);
  if (!editor.contains(range.commonAncestorContainer)) {
    return null;
  }
  return selection;
}

function getPointUnitOffset(
  editor: HTMLElement,
  container: Node,
  offset: number
): number {
  if (!editor.contains(container)) {
    return countUnitsInDomNode(editor);
  }
  const range = editor.ownerDocument.createRange();
  range.selectNodeContents(editor);
  try {
    range.setEnd(container, offset);
  } catch {
    return countUnitsInDomNode(editor);
  }
  const fragment = range.cloneContents();
  return countUnitsInDomNode(fragment);
}

function getSelectionUnitRange(editor: HTMLElement) {
  const selection = getSelectionInsideEditor(editor);
  const end = countUnitsInDomNode(editor);
  if (!selection || selection.rangeCount === 0) {
    return { start: end, end };
  }
  const range = selection.getRangeAt(0);
  const start = getPointUnitOffset(editor, range.startContainer, range.startOffset);
  const finish = getPointUnitOffset(editor, range.endContainer, range.endOffset);
  return start <= finish
    ? { start, end: finish }
    : { start: finish, end: start };
}

function resolveCaretDomPoint(editor: HTMLElement, unitOffset: number) {
  const totalUnits = countUnitsInDomNode(editor);
  let remaining = Math.max(0, Math.min(unitOffset, totalUnits));

  function walk(node: Node): { container: Node; offset: number } | null {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = node.textContent ?? "";
      if (remaining <= text.length) {
        return {
          container: node,
          offset: remaining,
        };
      }
      remaining -= text.length;
      return null;
    }

    if (isTokenChipNode(node)) {
      const parent = node.parentNode;
      if (!parent) return null;
      const index = Array.prototype.indexOf.call(parent.childNodes, node);
      if (remaining === 0) {
        return {
          container: parent,
          offset: index,
        };
      }
      remaining -= 1;
      if (remaining === 0) {
        return {
          container: parent,
          offset: index + 1,
        };
      }
      return null;
    }

    for (const child of Array.from(node.childNodes)) {
      const point = walk(child);
      if (point) return point;
    }
    return null;
  }

  const point = walk(editor);
  if (point) return point;
  return {
    container: editor,
    offset: editor.childNodes.length,
  };
}

function setEditorCaretByUnitOffset(editor: HTMLElement, unitOffset: number) {
  const selection = window.getSelection();
  if (!selection) return;
  const point = resolveCaretDomPoint(editor, unitOffset);
  const range = editor.ownerDocument.createRange();
  range.setStart(point.container, point.offset);
  range.collapse(true);
  selection.removeAllRanges();
  selection.addRange(range);
}

function placeEditorCaretAtEnd(editor: HTMLElement) {
  const selection = window.getSelection();
  if (!selection) return;
  const range = document.createRange();
  range.selectNodeContents(editor);
  range.collapse(false);
  selection.removeAllRanges();
  selection.addRange(range);
}

function placeEditorCaretFromPoint(editor: HTMLElement, x: number, y: number) {
  const doc = editor.ownerDocument;
  let range: Range | null = null;
  const docWithCaretRange = doc as Document & {
    caretRangeFromPoint?: (clientX: number, clientY: number) => Range | null;
  };

  try {
    if (typeof docWithCaretRange.caretRangeFromPoint === "function") {
      range = docWithCaretRange.caretRangeFromPoint(x, y);
    } else if (typeof doc.caretPositionFromPoint === "function") {
      const position = doc.caretPositionFromPoint(x, y);
      if (position) {
        range = doc.createRange();
        range.setStart(position.offsetNode, position.offset);
        range.collapse(true);
      }
    }
  } catch {
    return false;
  }

  if (!range || !editor.contains(range.startContainer)) {
    return false;
  }

  const selection = window.getSelection();
  if (!selection) return false;
  selection.removeAllRanges();
  selection.addRange(range);
  return true;
}

function splitSegmentsAtUnitOffset(segments: BuilderSegment[], unitOffset: number) {
  const clamped = Math.max(0, Math.min(unitOffset, countSegmentsUnits(segments)));
  let remaining = clamped;
  const left: BuilderSegment[] = [];
  const right: BuilderSegment[] = [];

  for (const segment of segments) {
    const segmentUnits = countSegmentUnits(segment);

    if (remaining <= 0) {
      right.push(segment);
      continue;
    }

    if (remaining >= segmentUnits) {
      left.push(segment);
      remaining -= segmentUnits;
      continue;
    }

    if (segment.kind === "text") {
      const leftText = segment.text.slice(0, remaining);
      const rightText = segment.text.slice(remaining);
      if (leftText.length) {
        left.push({
          ...segment,
          text: leftText,
        });
      }
      if (rightText.length) {
        right.push(createTextSegment(rightText));
      }
      remaining = 0;
      continue;
    }

    right.push(segment);
    remaining = 0;
  }

  return { left, right };
}

function replaceSegmentsUnitRange(
  segments: BuilderSegment[],
  start: number,
  end: number,
  inserted: BuilderSegment[]
) {
  const total = countSegmentsUnits(segments);
  const from = Math.max(0, Math.min(start, total));
  const to = Math.max(from, Math.min(end, total));

  const firstSplit = splitSegmentsAtUnitOffset(segments, from);
  const secondSplit = splitSegmentsAtUnitOffset(firstSplit.right, to - from);
  return ensureUniqueSegmentIds(
    mergeAdjacentTextSegments([
      ...firstSplit.left,
      ...inserted,
      ...secondSplit.right,
    ])
  );
}

function sliceSegmentsByUnitRange(
  segments: BuilderSegment[],
  start: number,
  end: number
) {
  const total = countSegmentsUnits(segments);
  const from = Math.max(0, Math.min(start, total));
  const to = Math.max(from, Math.min(end, total));
  const firstSplit = splitSegmentsAtUnitOffset(segments, from);
  const secondSplit = splitSegmentsAtUnitOffset(firstSplit.right, to - from);
  return ensureUniqueSegmentIds(mergeAdjacentTextSegments(secondSplit.left));
}

function parseClipboardSegments(raw: string): BuilderSegment[] | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as
      | {
          segments?: Array<{
            kind?: unknown;
            text?: unknown;
            token?: unknown;
            label?: unknown;
            category?: unknown;
          }>;
        }
      | null;
    if (!parsed || !Array.isArray(parsed.segments)) {
      return null;
    }
    const segments: BuilderSegment[] = [];
    for (const segment of parsed.segments) {
      if (segment.kind === "text" && typeof segment.text === "string") {
        segments.push(createTextSegment(segment.text));
        continue;
      }
      if (segment.kind === "token" && typeof segment.token === "string") {
        segments.push(
          createTokenSegment(
            segment.token,
            segment.category === "injector" ? "injector" : "event",
            typeof segment.label === "string" ? segment.label : segment.token
          )
        );
      }
    }
    return segments.length ? mergeAdjacentTextSegments(segments) : null;
  } catch {
    return null;
  }
}

function normalizeSpacerHeight(raw: unknown, fallback = 20) {
  if (typeof raw === "number" && Number.isFinite(raw)) {
    return Math.max(0, Math.round(raw));
  }

  if (typeof raw === "string") {
    const match = raw.match(/-?\d+(\.\d+)?/);
    if (match?.[0]) {
      const parsed = Number(match[0]);
      if (Number.isFinite(parsed)) {
        return Math.max(0, Math.round(parsed));
      }
    }
  }

  return fallback;
}

function parseVerticalAlign(value: string | null): "top" | "middle" | "bottom" {
  if (value === "top" || value === "middle" || value === "bottom") return value;
  return "middle";
}

function listStyleTypeForListBlock(listType: BuilderListBlock["listType"]): string {
  switch (listType) {
    case "bullet": return "disc";
    case "number": return "decimal";
    case "letter-upper": return "upper-alpha";
    case "letter-lower": return "lower-alpha";
    case "roman": return "upper-roman";
    default: return "disc";
  }
}

function createListItem(text: string = ""): ListItem {
  return {
    id: nowId(),
    segments: [createTextSegment(text)],
    children: [],
  };
}

function renderListItemsToHtml(
  items: ListItem[],
  listType: BuilderListBlock["listType"],
  depth: number = 0
): string {
  const tag = listType === "bullet" ? "ul" : "ol";
  const styleType = listStyleTypeForListBlock(listType);
  const liHtml = items
    .map((item) => {
      const textContent = renderSegmentsToText(item.segments).trim() || "&nbsp;";
      const nestedHtml =
        item.children.length > 0
          ? renderListItemsToHtml(item.children, listType, depth + 1)
          : "";
      return `<li>${textContent}${nestedHtml}</li>`;
    })
    .join("");
  return `<${tag} style="list-style-type:${styleType};padding-left:${depth === 0 ? 0 : 20}px;margin:0;">${liHtml}</${tag}>`;
}

function extractVideoThumbnail(url: string): string {
  if (!url) return "";
  // YouTube: youtube.com/watch?v=ID, youtu.be/ID, youtube.com/embed/ID
  const ytMatch = url.match(
    /(?:youtube\.com\/(?:watch\?.*v=|embed\/)|youtu\.be\/)([a-zA-Z0-9_-]{11})/
  );
  if (ytMatch?.[1]) {
    return `https://img.youtube.com/vi/${ytMatch[1]}/maxresdefault.jpg`;
  }
  // Vimeo: vimeo.com/ID
  const vimeoMatch = url.match(/vimeo\.com\/(\d+)/);
  if (vimeoMatch?.[1]) {
    return `https://vumbnail.com/${vimeoMatch[1]}.jpg`;
  }
  return "";
}

const VIDEO_THUMBNAIL_PATH = "/public/video-thumbnail";

/**
 * Wraps a raw thumbnail URL in the backend video-thumbnail composite endpoint
 * so the rendered email shows the thumbnail with a play-button overlay.
 */
function buildVideoThumbnailUrl(rawThumbnailUrl: string): string {
  if (!rawThumbnailUrl) return "";
  const apiBase =
    process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
  return `${apiBase}${VIDEO_THUMBNAIL_PATH}?url=${encodeURIComponent(rawThumbnailUrl)}`;
}

/**
 * Extracts the original thumbnail URL from a video-thumbnail composite endpoint
 * URL, so the editor can round-trip the thumbnail URL correctly.
 * If the src is not a composite URL, returns it as-is.
 */
function extractOriginalThumbnailUrl(src: string): string {
  if (!src) return "";
  try {
    const u = new URL(src);
    if (u.pathname === VIDEO_THUMBNAIL_PATH) {
      return u.searchParams.get("url") || src;
    }
  } catch {
    // Not a valid URL — might be a relative path or already a raw thumbnail URL.
    if (src.includes(VIDEO_THUMBNAIL_PATH + "?url=")) {
      const idx = src.indexOf("?url=");
      return decodeURIComponent(src.slice(idx + 5));
    }
  }
  return src;
}

function mergeAdjacentTextSegments(segments: BuilderSegment[]) {
  const merged: BuilderSegment[] = [];

  for (const segment of segments) {
    if (segment.kind === "text") {
      const last = merged[merged.length - 1];
      if (last?.kind === "text") {
        last.text += segment.text;
        continue;
      }
      merged.push({
        ...segment,
      });
      continue;
    }

    merged.push(segment);
  }

  const compact = merged.filter(
    (segment) => segment.kind !== "text" || segment.text.length > 0
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

function renderSegmentsToText(segments: BuilderSegment[]) {
  return segments
    .map((segment) =>
      segment.kind === "text"
        ? segment.text
        : variableToPlaceholder(segment.token)
    )
    .join("");
}

function renderSegmentsToEditorNode(editor: HTMLElement, segments: BuilderSegment[]) {
  const doc = editor.ownerDocument;
  const fragment = doc.createDocumentFragment();

  for (const segment of segments) {
    if (segment.kind === "text") {
      if (!segment.text) continue;
      fragment.appendChild(doc.createTextNode(segment.text));
      continue;
    }

    const chip = doc.createElement("span");
    chip.className = TOKEN_CHIP_CLASSNAME;
    chip.contentEditable = "false";
    chip.dataset.segmentKind = TOKEN_SEGMENT_KIND;
    chip.dataset.segmentId = segment.id;
    chip.dataset.token = segment.token;
    chip.dataset.label = segment.label;
    chip.dataset.category = segment.category;
    chip.textContent = variableToPlaceholder(segment.token);
    fragment.appendChild(chip);
  }

  editor.replaceChildren(fragment);
}

function normalizeBuilderDocument(raw: unknown): BuilderDocument {
  const fallback = {
    version: 1,
    blocks: [
      {
        id: nowId(),
        type: "text",
        content: "",
        align: "left" as const,
      },
    ],
  } satisfies BuilderDocument;

  if (!isRecord(raw)) {
    return fallback;
  }

  const doc = raw as Partial<BuilderDocument>;
  const blocksRaw = Array.isArray(doc.blocks) ? doc.blocks : null;
  if (!blocksRaw || !blocksRaw.length) {
    return fallback;
  }

  const blocks = blocksRaw
    .map((item): BuilderBlock | null => {
      if (!isRecord(item)) return null;
      const id =
        typeof item.id === "string" && item.id.trim() ? item.id : nowId();
      const label =
        typeof (item as { label?: unknown }).label === "string" && (item as { label?: string }).label!.trim()
          ? (item as { label: string }).label
          : undefined;
      const alignRaw = (item as { align?: unknown }).align;
      const alignFull: "left" | "center" | "right" | "justify" =
        alignRaw === "left" || alignRaw === "center" || alignRaw === "right" || alignRaw === "justify"
          ? alignRaw
          : "left";
      // Button/image blocks don't support justify
      const align: "left" | "center" | "right" =
        alignFull === "justify" ? "left" : alignFull;

      if (item.type === "text") {
        // New format: content is HTML string
        if (typeof (item as { content?: unknown }).content === "string") {
          return {
            id,
            label,
            type: "text",
            content: (item as { content: string }).content,
            align: alignFull,
          };
        }

        // Legacy format: convert segments to plain text HTML
        if (Array.isArray((item as { segments?: unknown }).segments)) {
          const incomingSegments = (
            (item as { segments?: unknown }).segments as unknown[]
          ).filter((segment): segment is BuilderSegment => isRecord(segment));
          const html = incomingSegments
            .map((segment) => {
              if (segment.kind === "text" && typeof segment.text === "string") {
                return segment.text.replace(/\n/g, "<br>");
              }
              if (segment.kind === "token" && typeof segment.token === "string") {
                const token = normalizeVariableToken(segment.token);
                const category = segment.category === "injector" ? "injector" : "event";
                const label = typeof segment.label === "string" ? segment.label : token;
                return `<span data-variable-token="${token}" data-category="${category}">${label}</span>`;
              }
              return "";
            })
            .join("");
          return {
            id,
            label,
            type: "text",
            content: html ? `<p>${html}</p>` : "",
            align: alignFull,
          };
        }

        return {
          id,
          label,
          type: "text",
          content: "",
          align: alignFull,
        };
      }
      if (item.type === "button") {
        if (Array.isArray((item as { segments?: unknown }).segments)) {
          const incomingSegments = (
            (item as { segments?: unknown }).segments as unknown[]
          ).filter((segment): segment is BuilderSegment => isRecord(segment));

          const sanitized = incomingSegments
            .map((segment) => {
              if (
                segment.kind === "text" &&
                typeof segment.id === "string" &&
                typeof segment.text === "string"
              ) {
                return {
                  kind: "text",
                  id: segment.id,
                  text: segment.text,
                } as BuilderSegment;
              }

              if (
                segment.kind === "token" &&
                typeof segment.id === "string" &&
                typeof segment.token === "string"
              ) {
                return {
                  kind: "token",
                  id: segment.id,
                  token: normalizeVariableToken(segment.token),
                  label:
                    typeof segment.label === "string"
                      ? segment.label
                      : segment.token,
                  category:
                    segment.category === "injector" ? "injector" : "event",
                } as BuilderSegment;
              }

              return null;
            })
            .filter((segment): segment is BuilderSegment => segment !== null);
          const uniqueSegments = ensureUniqueSegmentIds(sanitized);

          const initialHref =
            typeof item.href === "string" && item.href.trim() ? item.href : "#";
          return {
            id,
            label,
            type: "button",
            segments: uniqueSegments.length
              ? uniqueSegments
              : [createTextSegment("Button")],
            href: initialHref,
            align,
          };
        }

        const legacyContent =
          typeof (item as { content?: unknown }).content === "string"
            ? ((item as { content?: unknown }).content as string)
            : "Button";
        const href = typeof item.href === "string" ? item.href : "#";
        return {
          id,
          label,
          type: "button",
          segments: ensureUniqueSegmentIds(parseContentToSegments(legacyContent)),
          href,
          align,
        };
      }
      if (item.type === "image") {
        const src = typeof item.src === "string" ? item.src : "";
        const alt =
          typeof item.alt === "string" && item.alt.trim() ? item.alt : undefined;
        const width =
          typeof item.width === "string" && item.width.trim()
            ? item.width
            : undefined;
        return { id, label, type: "image", src, alt, width, align };
      }
      if (item.type === "divider") {
        return { id, label, type: "divider" };
      }
      if (item.type === "spacer") {
        const height = normalizeSpacerHeight(
          (item as { height?: unknown }).height,
          20
        );
        return { id, label, type: "spacer", height };
      }
      if (item.type === "banner") {
        const raw = item as Record<string, unknown>;
        return {
          id,
          label,
          type: "banner",
          backgroundUrl: typeof raw.backgroundUrl === "string" ? raw.backgroundUrl : "",
          backgroundColor: typeof raw.backgroundColor === "string" ? raw.backgroundColor : "#333333",
          mode: raw.mode === "fixed-height" ? "fixed-height" : "fluid-height",
          height: normalizeSpacerHeight(raw.height, 400),
          segments: Array.isArray(raw.segments)
            ? ensureUniqueSegmentIds(
                (raw.segments as unknown[])
                  .filter((s): s is BuilderSegment => isRecord(s))
                  .map((s) => {
                    if (s.kind === "text" && typeof s.text === "string")
                      return { kind: "text" as const, id: typeof s.id === "string" ? s.id : nowId(), text: s.text };
                    if (s.kind === "token" && typeof s.token === "string")
                      return {
                        kind: "token" as const,
                        id: typeof s.id === "string" ? s.id : nowId(),
                        token: normalizeVariableToken(s.token),
                        label: typeof s.label === "string" ? s.label : s.token,
                        category: s.category === "injector" ? "injector" as const : "event" as const,
                      };
                    return null;
                  })
                  .filter((s): s is BuilderSegment => s !== null)
              )
            : [createTextSegment("")],
          buttonText: typeof raw.buttonText === "string" ? raw.buttonText : "",
          buttonHref: typeof raw.buttonHref === "string" ? raw.buttonHref : "#",
          buttonColor: typeof raw.buttonColor === "string" ? raw.buttonColor : "#ffffff",
          verticalAlign: parseVerticalAlign(typeof raw.verticalAlign === "string" ? raw.verticalAlign : null),
          align,
          padding: normalizeSpacerHeight(raw.padding, 40),
        };
      }
      if (item.type === "video") {
        const raw = item as Record<string, unknown>;
        return {
          id,
          label,
          type: "video",
          videoUrl: typeof raw.videoUrl === "string" ? raw.videoUrl : "",
          thumbnailUrl: typeof raw.thumbnailUrl === "string" ? raw.thumbnailUrl : "",
          alt: typeof raw.alt === "string" ? raw.alt : "",
          width: typeof raw.width === "string" ? raw.width : "",
          align,
        };
      }
      if (item.type === "list") {
        const raw = item as Record<string, unknown>;
        const validListTypes = ["bullet", "number", "letter-upper", "letter-lower", "roman"];
        const listType: BuilderListBlock["listType"] =
          typeof raw.listType === "string" && validListTypes.includes(raw.listType)
            ? (raw.listType as BuilderListBlock["listType"])
            : "bullet";

        function normalizeListChildren(arr: unknown): ListItem[] {
          if (!Array.isArray(arr)) return [];
          return arr
            .filter((i): i is Record<string, unknown> => isRecord(i))
            .map((i): ListItem => ({
              id: typeof i.id === "string" ? i.id : nowId(),
              segments: Array.isArray(i.segments)
                ? ensureUniqueSegmentIds(
                    (i.segments as unknown[])
                      .filter((s): s is BuilderSegment => isRecord(s))
                      .map((s) => {
                        if (s.kind === "text" && typeof s.text === "string")
                          return { kind: "text" as const, id: typeof s.id === "string" ? s.id : nowId(), text: s.text };
                        if (s.kind === "token" && typeof s.token === "string")
                          return {
                            kind: "token" as const,
                            id: typeof s.id === "string" ? s.id : nowId(),
                            token: normalizeVariableToken(s.token),
                            label: typeof s.label === "string" ? s.label : s.token,
                            category: s.category === "injector" ? "injector" as const : "event" as const,
                          };
                        return null;
                      })
                      .filter((s): s is BuilderSegment => s !== null)
                  )
                : [createTextSegment("")],
              children: normalizeListChildren(i.children),
            }));
        }

        return {
          id,
          label,
          type: "list",
          listType,
          items: (() => {
            const parsed = normalizeListChildren(raw.items);
            return parsed.length > 0 ? parsed : [createListItem("")];
          })(),
          align,
        };
      }
      return null;
    })
    .filter((b): b is BuilderBlock => Boolean(b));

  if (!blocks.length) {
    return fallback;
  }

  return {
    version: 1,
    blocks,
  };
}

function normalizeVariableToken(raw: string) {
  return raw
    .replace(/^\s*\{\{\s*/, "")
    .replace(/\s*\}\}\s*$/, "")
    .replace(/\s+/g, " ")
    .trim();
}

function variableToPlaceholder(rawToken: string) {
  return `{{ ${normalizeVariableToken(rawToken)} }}`;
}

function sanitizePreviewHtml(rawHtml: string) {
  if (!rawHtml) return "";

  // String-based sanitization to avoid DOMParser→outerHTML round-trip which
  // breaks double-quoted CSS values inside style attributes (e.g. font-family:
  // "Courier New" gets truncated because outerHTML doesn't encode " as &quot;).
  let html = rawHtml;

  // Remove dangerous elements (with content)
  html = html.replace(
    /<(script|noscript|iframe|object|embed)\b[\s\S]*?<\/\1\s*>/gi,
    ""
  );
  // Remove self-closing dangerous elements
  html = html.replace(
    /<(script|noscript|iframe|object|embed)\b[^>]*\/?>/gi,
    ""
  );
  // Remove preload-script links
  html = html.replace(
    /<link\b[^>]*?rel\s*=\s*["']preload["'][^>]*?as\s*=\s*["']script["'][^>]*?\/?>/gi,
    ""
  );
  // Remove event-handler attributes (on*)
  html = html.replace(/\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*')/gi, "");
  // Remove javascript: URLs in src/href
  html = html.replace(
    /\s+(src|href|xlink:href)\s*=\s*(?:"javascript:[^"]*"|'javascript:[^']*')/gi,
    ""
  );

  return html;
}

function getPreviewScale(contentSize: PreviewDocumentSize, stage: PreviewStageSize) {
  const safeWidth = Math.max(1, contentSize.width);
  if (stage.width <= 0) {
    return 1;
  }
  return Math.max(
    MIN_PREVIEW_SCALE,
    Math.min(1, stage.width / safeWidth)
  );
}

function collectEventVariablesFromSchema(schema: unknown): string[] {
  if (!isRecord(schema)) {
    return [];
  }

  const tokens: string[] = [];
  const seen = new Set<string>();

  function walk(node: unknown, path: string) {
    if (!isRecord(node)) {
      if (path && !seen.has(path) && path !== "event") {
        seen.add(path);
        tokens.push(path);
      }
      return;
    }

    if (node.type === "object" && isRecord(node.properties)) {
      Object.entries(node.properties).forEach(([key, nested]) => {
        if (key.startsWith("$")) return;
        walk(nested, `${path}.${key}`);
      });
      return;
    }

    Object.entries(node).forEach(([key, nested]) => {
      if (
        key === "$schema" ||
        key === "type" ||
        key === "required" ||
        key === "properties" ||
        key === "description" ||
        key === "title" ||
        key === "additionalProperties" ||
        key === "items"
      ) {
        return;
      }
      const nextPath = `${path}.${key}`;
      if (isRecord(nested)) {
        walk(nested, nextPath);
      } else if (!seen.has(nextPath)) {
        seen.add(nextPath);
        tokens.push(nextPath);
      }
    });
  }

  const root = isRecord(schema.event) ? schema.event : schema;
  walk(root, "event");

  const normalized = Array.from(new Set(tokens))
    .map((token) => token.replace(/^event\.event\./, "event."))
    .filter((value) => value.length > "event.".length);

  return normalized;
}

function parseMjmlAlign(value: string | null): "left" | "center" | "right" | "justify" {
  if (value === "center" || value === "right" || value === "left" || value === "justify") {
    return value;
  }
  return "left";
}

function parseMjmlAlignNarrow(value: string | null): "left" | "center" | "right" {
  if (value === "center" || value === "right" || value === "left") {
    return value;
  }
  return "left";
}

function decodeMjmlEntities(raw: string) {
  return raw
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'");
}

function stripMjmlInlineTags(raw: string) {
  const withLineBreaks = raw
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/p\s*>/gi, "\n");
  const withoutTags = withLineBreaks.replace(/<[^>]+>/g, "");
  return decodeMjmlEntities(withoutTags).trim();
}

/** Convert {{event.x}} / {{injector.x}} in HTML to TipTap VariableToken spans */
function mjmlVarsToTiptapHtml(html: string): string {
  return html.replace(/\{\{([^}]+)\}\}/g, (_match, rawToken: string) => {
    const token = normalizeVariableToken(rawToken.trim());
    const category = guessSegmentCategory(token);
    const label = token;
    return `<span data-variable-token="${token}" data-category="${category}">${label}</span>`;
  });
}

/** Convert TipTap VariableToken spans back to {{token}} placeholders for MJML */
function tiptapHtmlToMjmlVars(html: string): string {
  return html.replace(
    /<span[^>]*data-variable-token="([^"]*)"[^>]*>[^<]*<\/span>/g,
    (_match, token: string) => `{{${token}}}`,
  );
}

function parseColumnChildToBlock(child: Element): BuilderBlock | null {
  const tag = child.tagName.toLowerCase();

  if (tag === "mj-text") {
    const rawInner = child.innerHTML ?? "";
    // Check if this is a list block (contains <ul> or <ol>)
    if (/<[ou]l[\s>]/i.test(rawInner)) {
      return parseMjTextListToBlock(child);
    }
    return {
      id: nowId(),
      type: "text",
      content: mjmlVarsToTiptapHtml(decodeMjmlEntities(rawInner)),
      align: parseMjmlAlign(child.getAttribute("align")),
    };
  }

  if (tag === "mj-button") {
    const content = stripMjmlInlineTags(child.innerHTML ?? "");
    return {
      id: nowId(),
      type: "button",
      segments: ensureUniqueSegmentIds(parseContentToSegments(content || "Button")),
      href: child.getAttribute("href") || "#",
      align: parseMjmlAlignNarrow(child.getAttribute("align")),
    };
  }

  if (tag === "mj-image") {
    const cssClass = child.getAttribute("css-class") || "";
    if (cssClass.includes("senda-video")) {
      return {
        id: nowId(),
        type: "video",
        videoUrl: child.getAttribute("href") || "",
        thumbnailUrl: extractOriginalThumbnailUrl(child.getAttribute("src") || ""),
        alt: child.getAttribute("alt") || "",
        width: child.getAttribute("width") || "",
        align: parseMjmlAlignNarrow(child.getAttribute("align")),
      };
    }
    return {
      id: nowId(),
      type: "image",
      src: child.getAttribute("src") || "",
      alt: child.getAttribute("alt") || undefined,
      width: child.getAttribute("width") || undefined,
      align: parseMjmlAlignNarrow(child.getAttribute("align")),
    };
  }

  if (tag === "mj-divider") {
    return { id: nowId(), type: "divider" };
  }

  if (tag === "mj-spacer") {
    return {
      id: nowId(),
      type: "spacer",
      height: normalizeSpacerHeight(child.getAttribute("height"), 20),
    };
  }

  return null;
}

function parseMjHeroToBlock(element: Element): BuilderBannerBlock {
  let segments: BuilderSegment[] = [createTextSegment("")];
  let buttonText = "";
  let buttonHref = "#";
  let buttonColor = "#ffffff";
  let align: "left" | "center" | "right" = "center";

  const textEl = element.getElementsByTagName("mj-text")[0];
  if (textEl) {
    segments = ensureUniqueSegmentIds(
      parseContentToSegments(stripMjmlInlineTags(textEl.innerHTML ?? ""))
    );
    align = parseMjmlAlignNarrow(textEl.getAttribute("align"));
  }

  const btnEl = element.getElementsByTagName("mj-button")[0];
  if (btnEl) {
    buttonText = stripMjmlInlineTags(btnEl.innerHTML ?? "");
    buttonHref = btnEl.getAttribute("href") || "#";
    buttonColor = btnEl.getAttribute("background-color") || "#ffffff";
  }

  return {
    id: nowId(),
    type: "banner",
    backgroundUrl: element.getAttribute("background-url") || "",
    backgroundColor: element.getAttribute("background-color") || "#333333",
    mode: element.getAttribute("mode") === "fixed-height" ? "fixed-height" : "fluid-height",
    height: normalizeSpacerHeight(element.getAttribute("height"), 400),
    segments,
    buttonText,
    buttonHref,
    buttonColor,
    verticalAlign: parseVerticalAlign(element.getAttribute("vertical-align")),
    align,
    padding: normalizeSpacerHeight(element.getAttribute("padding"), 40),
  };
}

function parseMjTextListToBlock(element: Element): BuilderListBlock {
  const rawInner = element.innerHTML ?? "";
  const parser = new DOMParser();
  const doc = parser.parseFromString(`<div>${rawInner}</div>`, "text/html");

  const listEl = doc.querySelector("ul, ol");
  const isOrdered = listEl?.tagName.toLowerCase() === "ol";
  const styleType =
    listEl?.getAttribute("style")?.match(/list-style-type:\s*([^;]+)/)?.[1]?.trim() || "";

  let listType: BuilderListBlock["listType"] = "bullet";
  if (isOrdered) {
    if (styleType === "upper-alpha") listType = "letter-upper";
    else if (styleType === "lower-alpha") listType = "letter-lower";
    else if (styleType === "upper-roman") listType = "roman";
    else listType = "number";
  }

  function parseListItems(parent: Element): ListItem[] {
    const items: ListItem[] = [];
    for (const li of Array.from(parent.children)) {
      if (li.tagName.toLowerCase() !== "li") continue;
      let textContent = "";
      for (const node of Array.from(li.childNodes)) {
        if (node.nodeType === Node.TEXT_NODE) {
          textContent += node.textContent || "";
        }
      }
      const nestedList = li.querySelector("ul, ol");
      const children = nestedList ? parseListItems(nestedList) : [];
      items.push({
        id: nowId(),
        segments: ensureUniqueSegmentIds(parseContentToSegments(textContent.trim())),
        children,
      });
    }
    return items;
  }

  const items = listEl ? parseListItems(listEl) : [createListItem("")];

  return {
    id: nowId(),
    type: "list",
    listType,
    items: items.length > 0 ? items : [createListItem("")],
    align: parseMjmlAlignNarrow(element.getAttribute("align")),
  };
}

function parseBuilderDocumentFromMjml(rawMjml: string): BuilderDocument | null {
  if (!rawMjml.trim()) {
    return null;
  }

  try {
    const parser = new DOMParser();
    const xmlDoc = parser.parseFromString(rawMjml, "text/xml");
    if (xmlDoc.querySelector("parsererror")) {
      return null;
    }

    const body = xmlDoc.getElementsByTagName("mj-body")[0];
    if (!body) {
      return null;
    }

    const blocks: BuilderBlock[] = [];

    for (const topChild of Array.from(body.children)) {
      const topTag = topChild.tagName.toLowerCase();

      if (topTag === "mj-hero") {
        blocks.push(parseMjHeroToBlock(topChild));
        continue;
      }

      if (topTag === "mj-section") {
        const column = topChild.getElementsByTagName("mj-column")[0];
        if (!column) continue;
        for (const child of Array.from(column.children)) {
          const parsed = parseColumnChildToBlock(child);
          if (parsed) blocks.push(parsed);
        }
      }
    }

    if (!blocks.length) {
      return null;
    }

    return {
      version: 1,
      blocks,
    };
  } catch {
    return null;
  }
}

function renderColumnBlockToMjml(block: BuilderBlock): string {
  switch (block.type) {
    case "text": {
      // Replace &quot; with ' so gomjml output doesn't produce unescaped "
      // inside style="..." attributes (breaks font-family: "Courier New" etc.)
      const inner = tiptapHtmlToMjmlVars(block.content).replace(/&quot;/g, "'").trim() || " ";
      const alignAttr = block.align !== "left" ? ` align="${block.align}"` : "";
      return `<mj-text${alignAttr}>${inner}</mj-text>`;
    }
    case "button":
      return `<mj-button href="${block.href || "#"}">${renderSegmentsToText(
        block.segments
      ).trim() || "Button"}</mj-button>`;
    case "image":
      return `\n<mj-image src="${block.src || ""}"${
        block.width ? ` width="${block.width}"` : ""
      }${block.alt ? ` alt="${block.alt}"` : ""} />`;
    case "divider":
      return "\n<mj-divider />";
    case "spacer":
      return `\n<mj-spacer height="${normalizeSpacerHeight(
        block.height,
        20
      )}px" />`;
    case "video": {
      const href = block.videoUrl ? ` href="${block.videoUrl}"` : "";
      const width = block.width ? ` width="${block.width}"` : "";
      const alt = block.alt ? ` alt="${block.alt}"` : "";
      const thumbSrc = block.thumbnailUrl
        ? buildVideoThumbnailUrl(block.thumbnailUrl)
        : "";
      return `\n<mj-image src="${thumbSrc}"${href}${width}${alt} align="${block.align}" css-class="senda-video" />`;
    }
    case "list": {
      const html = renderListItemsToHtml(block.items, block.listType);
      return `\n<mj-text align="${block.align}">${html}</mj-text>`;
    }
    default:
      return "";
  }
}

function renderBannerToMjml(block: BuilderBannerBlock): string {
  const modeAttr = ` mode="${block.mode}"`;
  const heightAttr = block.mode === "fixed-height" ? ` height="${block.height}px"` : "";
  const bgUrl = block.backgroundUrl ? ` background-url="${block.backgroundUrl}"` : "";
  const bgColor = ` background-color="${block.backgroundColor}"`;
  const vAlign = ` vertical-align="${block.verticalAlign}"`;
  const padding = ` padding="${block.padding}px"`;
  const textContent = renderSegmentsToText(block.segments).trim();
  const textMjml = textContent
    ? `\n        <mj-text align="${block.align}" color="#ffffff" font-size="20px">${textContent}</mj-text>`
    : "";
  const buttonMjml = block.buttonText
    ? `\n        <mj-button href="${block.buttonHref || "#"}" background-color="${block.buttonColor}" align="${block.align}">${block.buttonText}</mj-button>`
    : "";
  return `\n    <mj-hero${modeAttr}${heightAttr}${bgUrl}${bgColor}${vAlign}${padding}>${textMjml}${buttonMjml}\n    </mj-hero>`;
}

function buildTemplateMjml(document: BuilderDocument) {
  type BlockGroup =
    | { kind: "column"; blocks: BuilderBlock[] }
    | { kind: "hero"; block: BuilderBannerBlock };

  const groups: BlockGroup[] = [];
  let currentColumn: BuilderBlock[] = [];

  for (const block of document.blocks) {
    if (block.type === "banner") {
      if (currentColumn.length > 0) {
        groups.push({ kind: "column", blocks: currentColumn });
        currentColumn = [];
      }
      groups.push({ kind: "hero", block });
    } else {
      currentColumn.push(block);
    }
  }
  if (currentColumn.length > 0) {
    groups.push({ kind: "column", blocks: currentColumn });
  }

  const bodyContent = groups
    .map((group) => {
      if (group.kind === "hero") {
        return renderBannerToMjml(group.block);
      }
      const columnBlocks = group.blocks.map(renderColumnBlockToMjml).join("");
      return `\n    <mj-section>\n      <mj-column>${columnBlocks}\n      </mj-column>\n    </mj-section>`;
    })
    .join("");

  return `<mjml>\n  <mj-body>${bodyContent}\n  </mj-body>\n</mjml>`;
}

function makeVariableToken(name: string, category: "event" | "injector") {
  if (!name.trim()) return "";
  const cleanName = normalizeVariableToken(name);
  if (category === "event") {
    return `event.${cleanName.replace(/^event\./, "")}`;
  }
  return cleanName;
}

export function MjmlEditor() {
  const t = useTranslations("editor");

  const defaultBlockLabel: Record<BuilderBlockType, string> = {
    text: t("blocks.text"),
    button: t("blocks.button"),
    image: t("blocks.image"),
    divider: t("blocks.divider"),
    spacer: t("blocks.spacer"),
    banner: t("blocks.banner"),
    video: t("blocks.video"),
    list: t("blocks.list"),
  };

  const router = useRouter();
  const scope = useScope();
  const scopedPath = useScopedPath();
  const searchParams = useSearchParams();
  const params = useParams<{ slug: string }>();
  const templateTypeSlug = params.slug ?? "";
  const templateId = searchParams.get("templateId") ?? "";
  const versionId = searchParams.get("versionId") ?? "";

  const { data: version, isLoading: rawLoading } = useTemplateVersion(
    scopedPath,
    templateId,
    versionId
  );
  const isLoading = useMinimumLoading(rawLoading);

  const saveMutation = useSaveTemplateVersion(
    scopedPath,
    templateId,
    versionId
  );
  const publishMutation = usePublishVersion(
    scopedPath,
    templateId,
    versionId
  );
  const previewMutation = usePreviewMjml(scopedPath, templateId);

  const templateTypeQuery = useTemplateType(scopedPath, templateTypeSlug);
  const injectorList = useInjectorList(scopedPath);
  const api = useApi();
  const injectorItems = useMemo<InjectorDefinition[]>(() => {
    if (!injectorList.data) return [];
    return injectorList.data.pages.flatMap((page) => page.items);
  }, [injectorList.data]);
  const [injectorVariableTokens, setInjectorVariableTokens] = useState<
    TemplateVariable[]
  >([]);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [blocksOpen, setBlocksOpen] = useState(true);
  const [variablesOpen, setVariablesOpen] = useState(true);
  const [metadataOpen, setMetadataOpen] = useState(true);
  const [injectorGroupsOpen, setInjectorGroupsOpen] = useState<Record<string, boolean>>({});
  const [injectorSearch, setInjectorSearch] = useState("");

  const [editorMode, setEditorMode] = useState<EditorMode>("visual");
  const [previewHtml, setPreviewHtml] = useState("");
  const [previewFrameUrl, setPreviewFrameUrl] = useState("");
  const [showPublishConfirm, setShowPublishConfirm] = useState(false);
  const [showTestSend, setShowTestSend] = useState(false);
  const [activeLocale, setActiveLocale] = useState<string>("default");
  const activeLocaleRef = useRef(activeLocale);
  useEffect(() => { activeLocaleRef.current = activeLocale; }, [activeLocale]);
  const [addLocaleOpen, setAddLocaleOpen] = useState(false);
  const localeListQuery = useTemplateVersionLocales(scopedPath, templateId, versionId);
  const existingLocales = localeListQuery.data ?? [];
  const localeContentQuery = useTemplateLocale(
    scopedPath,
    templateId,
    versionId,
    activeLocale === "default" ? "" : activeLocale
  );
  const saveLocaleMutation = useSaveTemplateLocale(scopedPath, templateId, versionId);
  const deleteLocaleMutation = useDeleteTemplateLocale(scopedPath, templateId, versionId);
  const previewTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const resizeRef = useRef<{ startX: number; startWidth: number } | null>(
    null
  );

  const [builderDocument, setBuilderDocument] = useState<BuilderDocument | null>(
    null
  );
  const [codeOverride, setCodeOverride] = useState("");
  const [selectedBlockId, setSelectedBlockId] = useState<string | null>(null);
  const [collapsedBlocks, setCollapsedBlocks] = useState<Record<string, boolean>>({});
  const [previewSplitMode, setPreviewSplitMode] =
    useState<PreviewSplitMode>("ratio");
  const [previewPanelWidthPx, setPreviewPanelWidthPx] = useState<number>(
    MIN_PANEL_WIDTH
  );
  const [isResizeDragging, setIsResizeDragging] = useState(false);
  const [draggedBlockId, setDraggedBlockId] = useState<string | null>(null);
  const [blockDropIndex, setBlockDropIndex] = useState<number | null>(null);
  const draggedBlockIdRef = useRef<string | null>(null);
  const [draggedListItemId, setDraggedListItemId] = useState<string | null>(null);
  const [listItemDropTarget, setListItemDropTarget] = useState<{ itemId: string; position: "before" | "after" } | null>(null);
  const layoutSplitRef = useRef<HTMLDivElement | null>(null);
  const previewPanelWrapRef = useRef<HTMLDivElement | null>(null);
  const blockEditorRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const textBlockEditorRefs = useRef<Record<string, TextBlockEditorHandle | null>>({});
  const previewStageRef = useRef<HTMLDivElement | null>(null);
  const previewStageObserverRef = useRef<ResizeObserver | null>(null);
  const previewIframeRef = useRef<HTMLIFrameElement | null>(null);
  const previewObserverCleanupRef = useRef<(() => void) | null>(null);
  const [previewStageSize, setPreviewStageSize] = useState<PreviewStageSize>({
    width: 0,
    height: 0,
  });
  const [previewDocumentSize, setPreviewDocumentSize] =
    useState<PreviewDocumentSize>(DEFAULT_PREVIEW_DOCUMENT_SIZE);
  const pendingCaretRestoreRef = useRef<{
    blockId: string;
    unitOffset: number;
  } | null>(null);

  const {
    register,
    getValues,
    reset,
    formState: { errors },
  } = useForm<MetadataForm>({
    resolver: zodResolver(metadataSchema),
    values: version
      ? {
          subject: version.subject,
          preview_text: version.preview_text ?? "",
          from_name: version.from_name,
          reply_to: version.reply_to ?? "",
        }
      : {
          subject: "",
          preview_text: "",
          from_name: "",
          reply_to: "",
        },
  });

  const isDraft = version?.status === "draft";

  const autoSave = useAutoSave({
    getPayload: () => {
      const formData = getValues();
      const bodyPayload =
        editorMode === "visual"
          ? builderDocument
            ? buildTemplateMjml(builderDocument)
            : codeMjml
          : codeMjml;
      return {
        subject: formData.subject,
        preview_text: formData.preview_text || undefined,
        from_name: formData.from_name,
        reply_to: formData.reply_to || undefined,
        body_mjml: bodyPayload,
        default_locale: version?.default_locale ?? "en",
        ...(editorMode === "visual" && builderDocument
          ? { editor_data: builderDocument }
          : {}),
      };
    },
    saveFn: (data) => {
      if (activeLocaleRef.current === "default") {
        return saveMutation.mutateAsync(data);
      }
      // Save to locale endpoint
      return saveLocaleMutation.mutateAsync({
        locale: activeLocaleRef.current,
        subject: data.subject || undefined,
        preview_text: data.preview_text || undefined,
        from_name: data.from_name || undefined,
        body_mjml: data.body_mjml,
        ...(data.editor_data ? { editor_data: data.editor_data as Record<string, unknown> } : {}),
      });
    },
    enabled: isDraft,
    debounceMs: 2000,
  });

  async function handleSwitchLocale(locale: string) {
    if (locale === activeLocale) return;
    // Flush pending save
    await autoSave.save();
    setActiveLocale(locale);
    activeLocaleRef.current = locale;
  }

  useEffect(() => {
    if (!version) return;

    if (activeLocale === "default") {
      // Restore default version content
      const hasEditorData = Boolean(
        version.editor_data && Object.keys(version.editor_data).length > 0
      );
      const parsed =
        (hasEditorData
          ? normalizeBuilderDocument(version.editor_data)
          : parseBuilderDocumentFromMjml(version.body_mjml ?? "")) ??
        normalizeBuilderDocument(version.editor_data);
      if (parsed) setBuilderDocument(parsed);
      const code = version.body_mjml ?? "";
      setCodeOverride(code);
      if (code) triggerPreview(code);
      // Reset form to version values
      reset({
        subject: version.subject,
        preview_text: version.preview_text ?? "",
        from_name: version.from_name,
        reply_to: version.reply_to ?? "",
      });
      return;
    }

    // Non-default locale
    const localeData = localeContentQuery.data;
    if (localeData) {
      const hasEditorData = Boolean(
        localeData.editor_data && Object.keys(localeData.editor_data as object).length > 0
      );
      const parsed = hasEditorData
        ? normalizeBuilderDocument(localeData.editor_data)
        : parseBuilderDocumentFromMjml(localeData.body_mjml ?? version.body_mjml ?? "");
      if (parsed) setBuilderDocument(parsed);
      const code = localeData.body_mjml ?? version.body_mjml ?? "";
      setCodeOverride(code);
      if (code) triggerPreview(code);
      reset({
        subject: localeData.subject ?? version.subject,
        preview_text: localeData.preview_text ?? version.preview_text ?? "",
        from_name: localeData.from_name ?? version.from_name,
        reply_to: version.reply_to ?? "",
      });
    } else if (!localeContentQuery.isLoading) {
      // 404 or first load - use default content as starting point
      const hasEditorData = Boolean(
        version.editor_data && Object.keys(version.editor_data).length > 0
      );
      const parsed =
        (hasEditorData
          ? normalizeBuilderDocument(version.editor_data)
          : parseBuilderDocumentFromMjml(version.body_mjml ?? "")) ??
        normalizeBuilderDocument(version.editor_data);
      if (parsed) setBuilderDocument(parsed);
      const code = version.body_mjml ?? "";
      setCodeOverride(code);
      if (code) triggerPreview(code);
      reset({
        subject: version.subject,
        preview_text: version.preview_text ?? "",
        from_name: version.from_name,
        reply_to: version.reply_to ?? "",
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeLocale, localeContentQuery.data, localeContentQuery.isLoading]);

  async function handleAddLocale(locale: string) {
    setAddLocaleOpen(false);
    const formData = getValues();
    const bodyPayload =
      editorMode === "visual"
        ? builderDocument
          ? buildTemplateMjml(builderDocument)
          : codeOverride
        : codeOverride;
    try {
      await saveLocaleMutation.mutateAsync({
        locale,
        subject: formData.subject || undefined,
        preview_text: formData.preview_text || undefined,
        from_name: formData.from_name || undefined,
        body_mjml: bodyPayload,
      });
      await localeListQuery.refetch();
      await handleSwitchLocale(locale);
    } catch {
      toast.error(`Failed to create locale ${locale}`);
    }
  }

  async function handleDeleteLocale(locale: string) {
    try {
      // Switch away BEFORE delete to prevent autosave from re-creating the locale.
      if (activeLocale === locale) {
        activeLocaleRef.current = "default";
        setActiveLocale("default");
      }
      await deleteLocaleMutation.mutateAsync(locale);
    } catch {
      toast.error(`Failed to delete locale ${locale}`);
    }
  }

  const eventVariableTokens = useMemo<TemplateVariable[]>(() => {
    const raw = templateTypeQuery.data?.variable_schema;
    const extracted = collectEventVariablesFromSchema(raw as unknown);

    return Array.from(new Set(extracted)).map((name) => ({
        id: `event-${name}`,
        token: makeVariableToken(name, "event"),
        label: name,
        hint: t("variableHintEvent"),
        category: "event",
      }));
  }, [templateTypeQuery.data, t]);

  const templateVariables = useMemo(() => {
    return [...eventVariableTokens, ...injectorVariableTokens];
  }, [eventVariableTokens, injectorVariableTokens]);

  const triggerPreview = useCallback(
    (code: string) => {
      if (previewTimeoutRef.current) {
        clearTimeout(previewTimeoutRef.current);
      }
      previewTimeoutRef.current = setTimeout(async () => {
        if (!code.trim()) {
          return;
        }
        try {
          const result = await previewMutation.mutateAsync(code);
          setPreviewHtml(sanitizePreviewHtml(result.html));
        } catch {
          // Mantiene el comportamiento anterior: preview anterior si falla.
        }
      }, 800);
    },
    [previewMutation]
  );

  const serializedEditorData = useMemo(() => {
    if (!version?.editor_data) {
      return "";
    }

    try {
      return JSON.stringify(version.editor_data);
    } catch {
      return "";
    }
  }, [version?.editor_data]);

  useEffect(() => {
    if (!version) {
      return;
    }

    const hasPersistedEditorData = Boolean(
      version.editor_data && Object.keys(version.editor_data).length > 0
    );
    const parsed =
      (hasPersistedEditorData
        ? normalizeBuilderDocument(version.editor_data)
        : parseBuilderDocumentFromMjml(version.body_mjml ?? "")) ??
      normalizeBuilderDocument(version.editor_data);
    setBuilderDocument(parsed);

    const initialCode = version.body_mjml ?? "";
    const visualCode = buildTemplateMjml(parsed);
    const nextCode =
      typeof initialCode === "string" && initialCode.trim().length > 0
        ? initialCode
        : visualCode;
    setCodeOverride(nextCode);
    setSelectedBlockId(parsed.blocks[0]?.id ?? null);

    if (nextCode) {
      triggerPreview(nextCode);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version?.id, version?.body_mjml, serializedEditorData]);

  useEffect(() => {
    if (!scopedPath || !injectorItems.length) {
      setInjectorVariableTokens([]);
      return;
    }

    let cancelled = false;

    const injectorFields = [...new Set(injectorItems.map((item) => item.name))];
    const tokens: TemplateVariable[] = [];

    Promise.all(
      injectorFields.map((injectorName) =>
        api
          .get(`${scopedPath}/injectors/${injectorName}`)
          .json<InjectorWithValues>()
          .then((payload) => {
            const fields = Array.isArray(payload.fields) ? payload.fields : [];
            for (const field of fields) {
              const token = makeVariableToken(
                `injector.${injectorName}.${field.field_name}`,
                "injector"
              );
              if (!token) continue;
              tokens.push({
                id: `${injectorName}-${field.field_name}`,
                token,
                label: `${injectorName}.${field.field_name}`,
                hint: field.description || t("variableHintInjector"),
                category: "injector",
              });
            }
          })
          .catch(() => undefined)
      )
    )
      .then(() => {
        if (cancelled) {
          return;
        }
        const injectorTokens = tokens.filter((item, index, list) =>
          index === list.findIndex((candidate) => candidate.token === item.token)
        );

        setInjectorVariableTokens(injectorTokens);
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setInjectorVariableTokens([]);
      });

    return () => {
      cancelled = true;
    };
  }, [api, scopedPath, injectorItems, t]);

  function clampPreviewPanelWidth(nextWidth: number, containerWidth: number) {
    const safeContainerWidth = Math.max(0, Math.floor(containerWidth));
    const maxWidthByContainer = Math.max(
      MIN_PANEL_WIDTH,
      safeContainerWidth - PANEL_RESIZER_WIDTH - MIN_PANEL_WIDTH
    );
    return Math.max(MIN_PANEL_WIDTH, Math.min(maxWidthByContainer, nextWidth));
  }

  useEffect(() => {
    if (previewSplitMode !== "px") {
      return;
    }

    const container = layoutSplitRef.current;
    if (!container || typeof ResizeObserver === "undefined") {
      return;
    }

    const clampToContainer = (containerWidth: number) => {
      setPreviewPanelWidthPx((current) =>
        clampPreviewPanelWidth(current, containerWidth)
      );
    };

    clampToContainer(container.clientWidth);

    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (!rect) return;
      clampToContainer(rect.width);
    });

    observer.observe(container);
    return () => observer.disconnect();
  }, [previewSplitMode]);

  useEffect(() => {
    if (!isResizeDragging) {
      return;
    }

    const onMouseMove = (event: MouseEvent) => {
      if (!resizeRef.current) {
        return;
      }

      const delta = event.clientX - resizeRef.current.startX;
      const nextWidth = resizeRef.current.startWidth - delta;
      const containerWidth =
        layoutSplitRef.current?.clientWidth ??
        layoutSplitRef.current?.getBoundingClientRect().width ??
        0;
      const fallbackContainerWidth =
        resizeRef.current.startWidth * 2 + PANEL_RESIZER_WIDTH;
      const clamped = clampPreviewPanelWidth(
        nextWidth,
        containerWidth > 0 ? containerWidth : fallbackContainerWidth
      );

      setPreviewPanelWidthPx(clamped);
    };

    const onMouseUp = () => {
      resizeRef.current = null;
      setIsResizeDragging(false);
      document.body.style.userSelect = "";
      document.body.style.cursor = "";
    };

    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);

    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
      document.body.style.userSelect = "";
      document.body.style.cursor = "";
    };
  }, [isResizeDragging]);

  useEffect(() => {
    if (!previewHtml) {
      setPreviewFrameUrl("");
      setPreviewDocumentSize(DEFAULT_PREVIEW_DOCUMENT_SIZE);
      return;
    }

    const blob = new Blob([previewHtml], {
      type: "text/html;charset=utf-8",
    });
    const objectUrl = URL.createObjectURL(blob);
    setPreviewFrameUrl(objectUrl);

    return () => {
      URL.revokeObjectURL(objectUrl);
    };
  }, [previewHtml]);

  useEffect(() => {
    setPreviewDocumentSize(DEFAULT_PREVIEW_DOCUMENT_SIZE);
    previewObserverCleanupRef.current?.();
    previewObserverCleanupRef.current = null;
  }, [previewFrameUrl]);

  useEffect(() => {
    return () => {
      previewObserverCleanupRef.current?.();
      previewObserverCleanupRef.current = null;
    };
  }, []);

  const previewStageCallbackRef = useCallback((node: HTMLDivElement | null) => {
    previewStageRef.current = node;
    // Tear down previous observer
    if (previewStageObserverRef.current) {
      previewStageObserverRef.current.disconnect();
      previewStageObserverRef.current = null;
    }
    if (!node || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (!rect) return;
      setPreviewStageSize((current) => {
        const width = Math.max(0, Math.floor(rect.width));
        const height = Math.max(0, Math.floor(rect.height));
        if (current.width === width && current.height === height) {
          return current;
        }
        return { width, height };
      });
    });
    observer.observe(node);
    previewStageObserverRef.current = observer;
  }, []);

  useLayoutEffect(() => {
    if (!builderDocument || editorMode !== "visual") {
      return;
    }

    for (const block of builderDocument.blocks) {
      // Only sync segment-based blocks via contentEditable; text blocks use TipTap
      if (block.type !== "button" && block.type !== "banner") {
        continue;
      }

      const editor = blockEditorRefs.current[block.id];
      if (!editor) {
        continue;
      }

      const liveSegments = parseSegmentsFromEditorNode(editor);
      if (segmentsMeaningfullyEqual(liveSegments, block.segments)) {
        continue;
      }

      renderSegmentsToEditorNode(editor, block.segments);
    }
  }, [builderDocument, editorMode]);

  useLayoutEffect(() => {
    if (!builderDocument) {
      pendingCaretRestoreRef.current = null;
      return;
    }

    const pending = pendingCaretRestoreRef.current;
    if (!pending) return;

    const editor = blockEditorRefs.current[pending.blockId];
    if (!editor) {
      pendingCaretRestoreRef.current = null;
      return;
    }

    editor.focus();
    setEditorCaretByUnitOffset(editor, pending.unitOffset);
    pendingCaretRestoreRef.current = null;
  }, [builderDocument]);

  function startPreviewResize(event: ReactMouseEvent<HTMLButtonElement>) {
    if (event.button !== 0) return;
    event.preventDefault();
    const containerWidth =
      layoutSplitRef.current?.clientWidth ??
      layoutSplitRef.current?.getBoundingClientRect().width ??
      0;
    const measuredWrapperWidth =
      previewPanelWrapRef.current?.getBoundingClientRect().width;
    const fallbackWrapperWidth =
      previewSplitMode === "px"
        ? previewPanelWidthPx + PANEL_RESIZER_WIDTH
        : containerWidth > 0
          ? containerWidth * 0.5 + PANEL_RESIZER_WIDTH * 0.5
          : MIN_PANEL_WIDTH + PANEL_RESIZER_WIDTH;
    const resolvedWrapperWidth = measuredWrapperWidth ?? fallbackWrapperWidth;
    const requestedWidth = resolvedWrapperWidth - PANEL_RESIZER_WIDTH;
    const startWidth = clampPreviewPanelWidth(
      requestedWidth,
      containerWidth > 0
        ? containerWidth
        : requestedWidth * 2 + PANEL_RESIZER_WIDTH
    );

    resizeRef.current = {
      startX: event.clientX,
      startWidth,
    };
    setPreviewSplitMode("px");
    setPreviewPanelWidthPx(startWidth);
    document.body.style.userSelect = "none";
    document.body.style.cursor = "col-resize";
    setIsResizeDragging(true);
  }

  function readIframeDocumentSize(iframe: HTMLIFrameElement): PreviewDocumentSize {
    try {
      const doc = iframe.contentDocument;
      if (!doc) {
        return DEFAULT_PREVIEW_DOCUMENT_SIZE;
      }

      const body = doc.body;
      const root = doc.documentElement;
      const width = Math.max(
        body?.scrollWidth ?? 0,
        body?.offsetWidth ?? 0,
        body?.clientWidth ?? 0,
        root?.scrollWidth ?? 0,
        root?.offsetWidth ?? 0,
        root?.clientWidth ?? 0,
        1
      );
      const height = Math.max(
        body?.scrollHeight ?? 0,
        body?.offsetHeight ?? 0,
        body?.clientHeight ?? 0,
        root?.scrollHeight ?? 0,
        root?.offsetHeight ?? 0,
        root?.clientHeight ?? 0,
        1
      );

      return { width, height };
    } catch {
      return DEFAULT_PREVIEW_DOCUMENT_SIZE;
    }
  }

  function updatePreviewDocumentSize(next: PreviewDocumentSize) {
    setPreviewDocumentSize((current) => {
      const width = Math.max(1, Math.ceil(next.width));
      const height = Math.max(1, Math.ceil(next.height));
      if (
        Math.abs(current.width - width) <= 1 &&
        Math.abs(current.height - height) <= 1
      ) {
        return current;
      }
      return { width, height };
    });
  }

  function observeIframeDocumentSize(
    iframe: HTMLIFrameElement,
    onMeasure: (size: PreviewDocumentSize) => void
  ) {
    try {
      const doc = iframe.contentDocument;
      const frameWindow = iframe.contentWindow;
      if (!doc || !frameWindow || !doc.documentElement) {
        return () => undefined;
      }

      const measureNow = () => onMeasure(readIframeDocumentSize(iframe));
      const scheduleMeasure = () => {
        frameWindow.requestAnimationFrame(() => {
          measureNow();
        });
      };

      const resizeObserver =
        typeof ResizeObserver !== "undefined"
          ? new ResizeObserver(() => {
              scheduleMeasure();
            })
          : null;

      if (resizeObserver) {
        resizeObserver.observe(doc.documentElement);
        if (doc.body) {
          resizeObserver.observe(doc.body);
        }
      }

      const mutationObserver = new MutationObserver(() => {
        scheduleMeasure();
      });
      mutationObserver.observe(doc.documentElement, {
        childList: true,
        subtree: true,
        attributes: true,
        characterData: true,
      });

      doc.addEventListener("load", scheduleMeasure, true);
      frameWindow.addEventListener("resize", scheduleMeasure);

      measureNow();

      return () => {
        resizeObserver?.disconnect();
        mutationObserver.disconnect();
        doc.removeEventListener("load", scheduleMeasure, true);
        frameWindow.removeEventListener("resize", scheduleMeasure);
      };
    } catch {
      return () => undefined;
    }
  }

  function bindIframeSizeObserver(
    iframe: HTMLIFrameElement,
    cleanupRef: { current: (() => void) | null }
  ) {
    cleanupRef.current?.();
    cleanupRef.current = observeIframeDocumentSize(
      iframe,
      updatePreviewDocumentSize
    );
  }

  function handlePreviewIframeLoad(event: ReactSyntheticEvent<HTMLIFrameElement>) {
    bindIframeSizeObserver(event.currentTarget, previewObserverCleanupRef);
  }

  const codeMjml = version ? codeOverride : "";
  const previewNaturalWidth = Math.max(1, previewDocumentSize.width);
  const previewNaturalHeight = Math.max(1, previewDocumentSize.height);
  const previewScale = getPreviewScale(previewDocumentSize, previewStageSize);
  const previewScaledWidth = Math.max(
    1,
    Math.round(previewNaturalWidth * previewScale)
  );
  const previewScaledHeight = Math.max(
    1,
    Math.round(previewNaturalHeight * previewScale)
  );

  function updateBuilderDocument(next: BuilderDocument | null) {
    setBuilderDocument(next);
    const nextCode = next ? buildTemplateMjml(next) : "";
    setCodeOverride(nextCode);
    if (editorMode === "visual") {
      triggerPreview(nextCode);
    }
    autoSave.scheduleSave();
  }

  function handleCodeChange(value: string) {
    setCodeOverride(value);
    triggerPreview(value);
    autoSave.scheduleSave();
  }


  function handlePublish() {
    publishMutation
      .mutateAsync()
      .then(() => {
        toast.success("Version published");
        setShowPublishConfirm(false);
      })
      .catch(() => {
        toast.error("Failed to publish version");
      });
  }

  function buildBackPath() {
    const slug =
      typeof window !== "undefined"
        ? window.location.pathname.split("/templates/")[1]?.split("/")[0]
        : "";
    switch (scope.level) {
      case "global":
        return `/global/templates/${slug}`;
      case "tenant":
        return `/t/${scope.tenantCode}/templates/${slug}`;
      case "workspace":
        return `/t/${scope.tenantCode}/w/${scope.workspaceCode}/templates/${slug}`;
    }
  }

  function toggleBlockCollapsed(blockId: string) {
    setCollapsedBlocks((prev) => ({ ...prev, [blockId]: !prev[blockId] }));
  }

  function collapseAllBlocks() {
    if (!builderDocument) return;
    const next: Record<string, boolean> = {};
    for (const b of builderDocument.blocks) next[b.id] = true;
    setCollapsedBlocks(next);
  }

  function expandAllBlocks() {
    setCollapsedBlocks({});
  }

  function updateBlockLabel(blockId: string, label: string) {
    if (!builderDocument) return;
    const trimmed = label.trim();
    const block = builderDocument.blocks.find((b) => b.id === blockId);
    if (!block) return;
    const isDefault = !trimmed || trimmed === defaultBlockLabel[block.type];
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((b) =>
        b.id === blockId ? { ...b, label: isDefault ? undefined : trimmed } : b
      ),
    });
  }

  function addBlock(type: BuilderBlockType) {
    if (!builderDocument || !isDraft) return;

    const id = nowId();
    let newBlock: BuilderBlock;

    if (type === "text") {
      newBlock = {
        id,
        type: "text",
        content: "",
        align: "left",
      };
    } else if (type === "button") {
      newBlock = {
        id,
        type: "button",
        segments: [createTextSegment("Button")],
        href: "#",
        align: "center",
      };
    } else if (type === "image") {
      newBlock = {
        id,
        type: "image",
        src: "https://placehold.co/600x200",
        alt: "image",
        width: "100%",
        align: "left",
      };
    } else if (type === "divider") {
      newBlock = {
        id,
        type: "divider",
      };
    } else if (type === "banner") {
      newBlock = {
        id,
        type: "banner",
        backgroundUrl: "",
        backgroundColor: "#333333",
        mode: "fluid-height",
        height: 400,
        segments: [createTextSegment("Your headline here")],
        buttonText: "",
        buttonHref: "#",
        buttonColor: "#ffffff",
        verticalAlign: "middle",
        align: "center",
        padding: 40,
      };
    } else if (type === "video") {
      newBlock = {
        id,
        type: "video",
        videoUrl: "",
        thumbnailUrl: "https://placehold.co/600x340",
        alt: "Video thumbnail",
        width: "100%",
        align: "center",
      };
    } else if (type === "list") {
      newBlock = {
        id,
        type: "list",
        listType: "bullet",
        items: [
          createListItem("First item"),
          createListItem("Second item"),
          createListItem("Third item"),
        ],
        align: "left",
      };
    } else {
      newBlock = {
        id,
        type: "spacer",
        height: 20,
      };
    }

    updateBuilderDocument({
      ...builderDocument,
      blocks: [...builderDocument.blocks, newBlock],
    });
    setSelectedBlockId(id);
  }

  function removeBlock(blockId: string) {
    if (!builderDocument || !isDraft) return;

    const remaining = builderDocument.blocks.filter((block) => block.id !== blockId);
    if (!remaining.length) {
      const fallback: BuilderDocument = {
        version: 1,
        blocks: [
          {
            id: nowId(),
            type: "text",
            content: "",
            align: "left",
          },
        ],
      };
      updateBuilderDocument(fallback);
      setSelectedBlockId(fallback.blocks[0].id);
      return;
    }

    updateBuilderDocument({
      ...builderDocument,
      blocks: remaining,
    });

    if (selectedBlockId === blockId) {
      setSelectedBlockId(remaining[0]?.id ?? null);
    }
  }

  function getButtonBlockSegments(blockId: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find((candidate) => candidate.id === blockId);
    if (!block || block.type !== "button") {
      return null;
    }
    return block.segments;
  }

  function updateButtonBlockSegments(
    blockId: string,
    nextSegments: BuilderSegment[],
    nextCaretUnitOffset?: number
  ) {
    if (!builderDocument) return;
    const normalized = ensureUniqueSegmentIds(mergeAdjacentTextSegments(nextSegments));
    if (typeof nextCaretUnitOffset === "number") {
      const clampedCaret = Math.max(
        0,
        Math.min(nextCaretUnitOffset, countSegmentsUnits(normalized))
      );
      pendingCaretRestoreRef.current = {
        blockId,
        unitOffset: clampedCaret,
      };
    }
    const current = getButtonBlockSegments(blockId);
    if (current && segmentsMeaningfullyEqual(current, normalized)) {
      if (typeof nextCaretUnitOffset === "number") {
        const editor = blockEditorRefs.current[blockId];
        if (editor) {
          setEditorCaretByUnitOffset(editor, nextCaretUnitOffset);
        }
      }
      pendingCaretRestoreRef.current = null;
      return;
    }
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "button") {
          return block;
        }
        return {
          ...block,
          segments: normalized,
        };
      }),
    });
  }

  function updateTextBlock(
    blockId: string,
    html: string,
    newAlign?: "left" | "center" | "right" | "justify"
  ) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "text") return block;
        return {
          ...block,
          content: html,
          ...(newAlign !== undefined && { align: newAlign }),
        };
      }),
    });
  }

  function handleBlockEditorInput(blockId: string, editor: HTMLDivElement) {
    const parsed = parseSegmentsFromEditorNode(editor);
    const block = builderDocument?.blocks.find((b) => b.id === blockId);
    if (block?.type === "banner") {
      updateBannerBlockSegments(blockId, parsed);
    } else {
      updateButtonBlockSegments(blockId, parsed);
    }
  }

  function insertVariableIntoSegmentEditor(
    blockId: string,
    editor: HTMLDivElement,
    variable: TemplateVariable
  ) {
    const token = normalizeVariableToken(variable.token);
    if (!token) {
      return false;
    }
    const tokenSegment = createTokenSegment(token, variable.category, variable.label);
    const liveSegments = parseSegmentsFromEditorNode(editor);
    const selection = getSelectionUnitRange(editor);
    const nextSegments = replaceSegmentsUnitRange(
      liveSegments,
      selection.start,
      selection.end,
      [tokenSegment]
    );
    if (!nextSegments.length) {
      return false;
    }
    const block = builderDocument?.blocks.find((b) => b.id === blockId);
    if (block?.type === "banner") {
      updateBannerBlockSegments(blockId, nextSegments, selection.start + 1);
    } else {
      updateButtonBlockSegments(blockId, nextSegments, selection.start + 1);
    }
    return true;
  }

  function appendTemplateVariableToBlock(
    blockId: string,
    variable: TemplateVariable
  ) {
    if (!builderDocument || !isDraft) return;

    const targetBlock =
      builderDocument.blocks.find((block) => block.id === blockId) ??
      builderDocument.blocks.find(
        (block) => block.type === "text" || block.type === "button" || block.type === "banner"
      ) ??
      builderDocument.blocks[0];

    if (!targetBlock) return;
    const targetId = targetBlock.id;

    setSelectedBlockId(targetId);

    const token = normalizeVariableToken(variable.token);
    if (!token) return;

    // Text blocks: use TipTap editor ref
    if (targetBlock.type === "text") {
      const tiptapRef = textBlockEditorRefs.current[targetId];
      if (tiptapRef) {
        tiptapRef.insertVariable({
          token,
          label: variable.label,
          category: variable.category,
        });
      }
      return;
    }

    // Button / Banner blocks: use contentEditable segment approach
    const editor = blockEditorRefs.current[targetId];
    if (editor && insertVariableIntoSegmentEditor(targetId, editor, variable)) {
      return;
    }

    const tokenSegment = createTokenSegment(token, variable.category, variable.label);
    if (targetBlock.type === "banner") {
      const currentSegments = getBannerBlockSegments(targetId);
      if (!currentSegments) return;
      const end = countSegmentsUnits(currentSegments);
      const nextSegments = replaceSegmentsUnitRange(currentSegments, end, end, [tokenSegment]);
      updateBannerBlockSegments(targetId, nextSegments, end + 1);
      return;
    }
    const currentSegments = getButtonBlockSegments(targetId);
    if (!currentSegments) return;
    const end = countSegmentsUnits(currentSegments);
    const nextSegments = replaceSegmentsUnitRange(currentSegments, end, end, [
      tokenSegment,
    ]);
    updateButtonBlockSegments(targetId, nextSegments, end + 1);
  }

  function handleBlockEditorTokenClick(event: ReactMouseEvent<HTMLElement>) {
    const target = event.target as HTMLElement | null;
    if (!target || !target.dataset.segmentKind) {
      return;
    }
    if (target.dataset.segmentKind !== TOKEN_SEGMENT_KIND) {
      return;
    }
    const selection = window.getSelection();
    if (!selection) return;
    const range = document.createRange();
    range.selectNode(target);
    selection.removeAllRanges();
    selection.addRange(range);
  }

  function handleBlockEditorKeyDown(
    blockId: string,
    event: ReactKeyboardEvent<HTMLDivElement>
  ) {
    if (!isDraft) return;
    if (event.key === "Enter") {
      event.preventDefault();
      return;
    }
    if (event.key !== "Backspace" && event.key !== "Delete") {
      return;
    }

    const editor = event.currentTarget;
    const liveSegments = parseSegmentsFromEditorNode(editor);
    const selectionRange = getSelectionUnitRange(editor);
    const totalUnits = countSegmentsUnits(liveSegments);
    const hasSelection = selectionRange.start !== selectionRange.end;

    let deleteStart = selectionRange.start;
    let deleteEnd = selectionRange.end;

    if (!hasSelection) {
      if (event.key === "Backspace") {
        if (selectionRange.start <= 0) return;
        deleteStart = selectionRange.start - 1;
        deleteEnd = selectionRange.start;
      } else {
        if (selectionRange.start >= totalUnits) return;
        deleteStart = selectionRange.start;
        deleteEnd = selectionRange.start + 1;
      }
    }

    if (deleteStart === deleteEnd) {
      return;
    }

    event.preventDefault();
    const nextSegments = replaceSegmentsUnitRange(
      liveSegments,
      deleteStart,
      deleteEnd,
      []
    );
    const targetBlock = builderDocument?.blocks.find((b) => b.id === blockId);
    if (targetBlock?.type === "banner") {
      updateBannerBlockSegments(blockId, nextSegments, deleteStart);
    } else {
      updateButtonBlockSegments(blockId, nextSegments, deleteStart);
    }
  }

  function handleBlockEditorCopyOrCut(
    blockId: string,
    event: ReactClipboardEvent<HTMLDivElement>,
    mode: "copy" | "cut"
  ) {
    const editor = event.currentTarget;
    const liveSegments = parseSegmentsFromEditorNode(editor);
    const selectionRange = getSelectionUnitRange(editor);
    if (selectionRange.start === selectionRange.end) {
      return;
    }
    const copiedSegments = sliceSegmentsByUnitRange(
      liveSegments,
      selectionRange.start,
      selectionRange.end
    );
    const plain = renderSegmentsToText(copiedSegments);

    event.preventDefault();
    event.clipboardData.setData("text/plain", plain);
    event.clipboardData.setData(
      CLIPBOARD_SEGMENTS_MIME,
      JSON.stringify({
        segments: copiedSegments.map((segment) =>
          segment.kind === "text"
            ? { kind: "text", text: segment.text }
            : {
                kind: "token",
                token: segment.token,
                label: segment.label,
                category: segment.category,
              }
        ),
      })
    );
    if (mode === "cut" && isDraft) {
      const nextSegments = replaceSegmentsUnitRange(
        liveSegments,
        selectionRange.start,
        selectionRange.end,
        []
      );
      const targetBlock = builderDocument?.blocks.find((b) => b.id === blockId);
      if (targetBlock?.type === "banner") {
        updateBannerBlockSegments(blockId, nextSegments, selectionRange.start);
      } else {
        updateButtonBlockSegments(blockId, nextSegments, selectionRange.start);
      }
    }
  }

  function handleBlockEditorPaste(
    blockId: string,
    event: ReactClipboardEvent<HTMLDivElement>
  ) {
    if (!isDraft) {
      return;
    }
    const editor = event.currentTarget;
    const clipboardSegments = parseClipboardSegments(
      event.clipboardData.getData(CLIPBOARD_SEGMENTS_MIME)
    );
    const fallbackText = event.clipboardData.getData("text/plain");
    const textPayload =
      clipboardSegments && clipboardSegments.length > 0
        ? renderSegmentsToText(clipboardSegments)
        : fallbackText;
    if (!textPayload) {
      return;
    }

    event.preventDefault();
    const liveSegments = parseSegmentsFromEditorNode(editor);
    const selectionRange = getSelectionUnitRange(editor);
    const insertedSegments = parseTextChunkToSegments(textPayload);
    const nextSegments = replaceSegmentsUnitRange(
      liveSegments,
      selectionRange.start,
      selectionRange.end,
      insertedSegments
    );
    const nextCaret =
      selectionRange.start + countSegmentsUnits(insertedSegments);
    const targetBlock = builderDocument?.blocks.find((b) => b.id === blockId);
    if (targetBlock?.type === "banner") {
      updateBannerBlockSegments(blockId, nextSegments, nextCaret);
    } else {
      updateButtonBlockSegments(blockId, nextSegments, nextCaret);
    }
  }

  function handleBlockEditorDragOver(
    blockId: string,
    event: DragEvent<HTMLDivElement>
  ) {
    if (!isDraft || isBlockDragEvent(event.dataTransfer)) {
      return;
    }
    if (!isVariableDragEvent(event.dataTransfer)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = "copy";
    const editor = event.currentTarget;
    editor.focus();
    placeEditorCaretFromPoint(editor, event.clientX, event.clientY);
    setSelectedBlockId(blockId);
  }

  function handleBlockEditorDrop(
    blockId: string,
    event: DragEvent<HTMLDivElement>
  ) {
    if (!isDraft || isBlockDragEvent(event.dataTransfer)) {
      return;
    }
    if (!isVariableDragEvent(event.dataTransfer)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const editor = event.currentTarget;
    editor.focus();
    if (!placeEditorCaretFromPoint(editor, event.clientX, event.clientY)) {
      placeEditorCaretAtEnd(editor);
    }
    const variable = resolveVariableFromDrop(event);
    if (!variable) {
      return;
    }
    insertVariableIntoSegmentEditor(blockId, editor, variable);
    setSelectedBlockId(blockId);
  }

  function isBlockDragEvent(dataTransfer: DataTransfer) {
    if (draggedBlockIdRef.current) return true;
    if (Array.from(dataTransfer.types).includes(BLOCK_DND_MIME)) {
      return true;
    }
    const plain = dataTransfer.getData("text/plain");
    return plain.startsWith("block:");
  }

  function isVariableDragEvent(dataTransfer: DataTransfer) {
    if (isBlockDragEvent(dataTransfer)) return false;
    const types = Array.from(dataTransfer.types);
    return (
      types.includes(VARIABLE_DND_MIME) ||
      types.includes("application/json") ||
      types.includes("text/plain")
    );
  }

  function resolveVariableFromDrop(event: DragEvent<HTMLElement>): TemplateVariable | null {
    if (event.dataTransfer.getData(BLOCK_DND_MIME)) {
      return null;
    }

    const rawJson =
      event.dataTransfer.getData(VARIABLE_DND_MIME) ||
      event.dataTransfer.getData("application/json");
    if (rawJson) {
      try {
        const parsed = JSON.parse(rawJson) as
          | {
              token?: unknown;
              label?: unknown;
              category?: unknown;
            }
          | null;

        if (parsed && typeof parsed.token === "string" && parsed.token.trim()) {
          const token = normalizeVariableToken(parsed.token);
          const label =
            typeof parsed.label === "string" && parsed.label.trim()
              ? parsed.label
              : token;
          const category =
            parsed.category === "injector" ? "injector" : "event";

          if (token) {
            return {
              id: nowId(),
              token,
              label,
              hint: "",
              category,
            };
          }
        }
      } catch {
        // no-op
      }
    }

    const token = normalizeVariableToken(event.dataTransfer.getData("text/plain"));
    if (!token || !token.includes(".")) return null;

    return {
      id: nowId(),
      token,
      label: token,
      hint: "",
      category: guessSegmentCategory(token),
    };
  }

  function updateImageBlock(
    blockId: string,
    key: "src" | "alt" | "width",
    value: string
  ) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "image") return block;
        return {
          ...block,
          [key]: value,
        };
      }),
    });
  }

  function updateSpacer(blockId: string, height: number) {
    if (!builderDocument) return;
    const normalizedHeight = normalizeSpacerHeight(height, 0);
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "spacer") return block;
        return {
          ...block,
          height: normalizedHeight,
        };
      }),
    });
  }

  function updateButtonHref(blockId: string, href: string) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "button") return block;
        return {
          ...block,
          href,
        };
      }),
    });
  }

  function updateBannerBlock(
    blockId: string,
    key: keyof Omit<BuilderBannerBlock, "id" | "type" | "segments">,
    value: string | number
  ) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "banner") return block;
        return { ...block, [key]: value };
      }),
    });
  }

  function getBannerBlockSegments(blockId: string) {
    if (!builderDocument) return null;
    const block = builderDocument.blocks.find((b) => b.id === blockId);
    if (!block || block.type !== "banner") return null;
    return block.segments;
  }

  function updateBannerBlockSegments(
    blockId: string,
    nextSegments: BuilderSegment[],
    nextCaretUnitOffset?: number
  ) {
    if (!builderDocument) return;
    const normalized = ensureUniqueSegmentIds(mergeAdjacentTextSegments(nextSegments));
    if (typeof nextCaretUnitOffset === "number") {
      const clampedCaret = Math.max(
        0,
        Math.min(nextCaretUnitOffset, countSegmentsUnits(normalized))
      );
      pendingCaretRestoreRef.current = {
        blockId,
        unitOffset: clampedCaret,
      };
    }
    const current = getBannerBlockSegments(blockId);
    if (current && segmentsMeaningfullyEqual(current, normalized)) {
      if (typeof nextCaretUnitOffset === "number") {
        const editor = blockEditorRefs.current[blockId];
        if (editor) {
          setEditorCaretByUnitOffset(editor, nextCaretUnitOffset);
        }
      }
      pendingCaretRestoreRef.current = null;
      return;
    }
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "banner") return block;
        return { ...block, segments: normalized };
      }),
    });
  }

  function updateVideoBlock(
    blockId: string,
    key: keyof Omit<BuilderVideoBlock, "id" | "type">,
    value: string
  ) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "video") return block;
        return { ...block, [key]: value };
      }),
    });
  }

  function updateListBlock(
    blockId: string,
    key: "listType" | "align",
    value: string
  ) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "list") return block;
        return { ...block, [key]: value };
      }),
    });
  }

  function updateListItems(blockId: string, items: ListItem[]) {
    if (!builderDocument) return;
    updateBuilderDocument({
      ...builderDocument,
      blocks: builderDocument.blocks.map((block) => {
        if (block.id !== blockId || block.type !== "list") return block;
        return { ...block, items };
      }),
    });
  }

  function addListItem(blockId: string, afterItemId?: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    const newItem = createListItem("");
    if (!afterItemId) {
      updateListItems(blockId, [...block.items, newItem]);
      return;
    }

    function insertAfter(items: ListItem[]): ListItem[] {
      const result: ListItem[] = [];
      for (const item of items) {
        result.push({ ...item, children: insertAfter(item.children) });
        if (item.id === afterItemId) {
          result.push(newItem);
        }
      }
      return result;
    }
    updateListItems(blockId, insertAfter(block.items));
  }

  function removeListItem(blockId: string, itemId: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    function removeFromItems(items: ListItem[]): ListItem[] {
      return items
        .filter((item) => item.id !== itemId)
        .map((item) => ({ ...item, children: removeFromItems(item.children) }));
    }

    const remaining = removeFromItems(block.items);
    updateListItems(blockId, remaining.length > 0 ? remaining : [createListItem("")]);
  }

  function indentListItem(blockId: string, itemId: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    function indentInItems(items: ListItem[]): ListItem[] {
      const result: ListItem[] = [];
      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.id === itemId && i > 0) {
          const prev = result[result.length - 1];
          result[result.length - 1] = {
            ...prev,
            children: [...prev.children, item],
          };
        } else {
          result.push({ ...item, children: indentInItems(item.children) });
        }
      }
      return result;
    }
    updateListItems(blockId, indentInItems(block.items));
  }

  function outdentListItem(blockId: string, itemId: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    function outdentInItems(items: ListItem[]): ListItem[] {
      const result: ListItem[] = [];
      for (const item of items) {
        const targetIndex = item.children.findIndex((c) => c.id === itemId);
        if (targetIndex >= 0) {
          const before = item.children.slice(0, targetIndex);
          const target = item.children[targetIndex];
          const after = item.children.slice(targetIndex + 1);
          result.push({ ...item, children: [...before, ...after] });
          result.push({ ...target });
        } else {
          result.push({ ...item, children: outdentInItems(item.children) });
        }
      }
      return result;
    }
    updateListItems(blockId, outdentInItems(block.items));
  }

  function updateListItemSegments(blockId: string, itemId: string, text: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    const newSegments = parseContentToSegments(text);
    function updateItem(items: ListItem[]): ListItem[] {
      return items.map((i) =>
        i.id === itemId
          ? { ...i, segments: newSegments }
          : { ...i, children: updateItem(i.children) }
      );
    }
    updateListItems(blockId, updateItem(block.items));
  }

  function moveListItem(blockId: string, draggedItemId: string, targetItemId: string, position: "before" | "after") {
    if (!builderDocument || !isDraft || draggedItemId === targetItemId) return;
    const block = builderDocument.blocks.find(
      (b) => b.id === blockId && b.type === "list"
    );
    if (!block || block.type !== "list") return;

    // 1. Extract the dragged item from the tree
    let extracted: ListItem | null = null;
    function removeItem(items: ListItem[]): ListItem[] {
      return items
        .filter((item) => {
          if (item.id === draggedItemId) {
            extracted = item;
            return false;
          }
          return true;
        })
        .map((item) => ({ ...item, children: removeItem(item.children) }));
    }
    const withoutDragged = removeItem(block.items);
    if (!extracted) return;

    // 2. Insert at the target position among its siblings
    function insertAtTarget(items: ListItem[]): ListItem[] {
      const result: ListItem[] = [];
      for (const item of items) {
        if (item.id === targetItemId) {
          if (position === "before") {
            result.push(extracted!);
            result.push({ ...item, children: insertAtTarget(item.children) });
          } else {
            result.push({ ...item, children: insertAtTarget(item.children) });
            result.push(extracted!);
          }
        } else {
          result.push({ ...item, children: insertAtTarget(item.children) });
        }
      }
      return result;
    }
    const reordered = insertAtTarget(withoutDragged);
    updateListItems(blockId, reordered.length > 0 ? reordered : [createListItem("")]);
  }

  function onVariableCardDragStart(
    event: DragEvent<HTMLButtonElement>,
    variable: TemplateVariable
  ) {
    draggedBlockIdRef.current = null;
    setDraggedBlockId(null);
    const safeToken = normalizeVariableToken(variable.token);
    if (!safeToken) return;
    const payload = {
      token: safeToken,
      label: variable.label,
      category: variable.category,
    };
    event.dataTransfer.setData(VARIABLE_DND_MIME, JSON.stringify(payload));
    event.dataTransfer.setData("application/json", JSON.stringify(payload));
    event.dataTransfer.setData("text/plain", safeToken);
    event.dataTransfer.effectAllowed = "copy";
  }

  function onBlockHandleDragStart(
    event: DragEvent<HTMLButtonElement>,
    blockId: string
  ) {
    if (!isDraft) return;
    event.dataTransfer.setData(BLOCK_DND_MIME, blockId);
    event.dataTransfer.setData("text/plain", `block:${blockId}`);
    event.dataTransfer.effectAllowed = "move";
    draggedBlockIdRef.current = blockId;
    setDraggedBlockId(blockId);
  }

  function getDraggedBlockId(dataTransfer: DataTransfer) {
    const blockId = dataTransfer.getData(BLOCK_DND_MIME);
    if (blockId?.trim()) {
      return blockId;
    }
    return draggedBlockIdRef.current || draggedBlockId;
  }

  function handleBlockDropZoneDragOver(
    event: DragEvent<HTMLDivElement>,
    dropIndex: number
  ) {
    if (!isDraft || !builderDocument) return;
    if (!isBlockDragEvent(event.dataTransfer)) return;

    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setBlockDropIndex(dropIndex);
  }

  function moveBlockToIndex(blockId: string, targetIndex: number) {
    if (!builderDocument || !isDraft) return;
    const sourceIndex = builderDocument.blocks.findIndex(
      (block) => block.id === blockId
    );
    if (sourceIndex < 0) {
      return;
    }

    const boundedTarget = Math.max(
      0,
      Math.min(targetIndex, builderDocument.blocks.length)
    );
    const nextIndex =
      sourceIndex < boundedTarget ? boundedTarget - 1 : boundedTarget;
    if (nextIndex === sourceIndex) {
      return;
    }

    const nextBlocks = [...builderDocument.blocks];
    const [movedBlock] = nextBlocks.splice(sourceIndex, 1);
    if (!movedBlock) return;
    nextBlocks.splice(nextIndex, 0, movedBlock);
    updateBuilderDocument({
      ...builderDocument,
      blocks: nextBlocks,
    });
    setSelectedBlockId(blockId);
  }

  function handleBlockDropZoneDrop(
    event: DragEvent<HTMLDivElement>,
    dropIndex: number
  ) {
    if (!isDraft) return;
    const blockId = getDraggedBlockId(event.dataTransfer);
    if (!blockId) return;

    event.preventDefault();
    moveBlockToIndex(blockId, dropIndex);
    draggedBlockIdRef.current = null;
    setDraggedBlockId(null);
    setBlockDropIndex(null);
  }

  function handleBlockDragEnd() {
    draggedBlockIdRef.current = null;
    setDraggedBlockId(null);
    setBlockDropIndex(null);
  }

  if (isLoading) {
    return <MjmlEditorSkeleton />;
  }

  if (!version) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Version not found
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full overflow-hidden animate-in fade-in duration-300">
      {/* Header bar */}
      <div className="flex flex-col gap-2 px-6 py-3 border-b bg-card shrink-0">
        <div className="flex items-center justify-between h-14">
          <div className="flex items-center gap-3">
            <button
              onClick={() => router.push(buildBackPath())}
              className="text-muted-foreground hover:text-foreground"
            >
              <ArrowLeft className="h-5 w-5" />
            </button>
            <div className="flex items-center gap-2 text-sm">
              <span className="text-muted-foreground">Templates</span>
              <span className="text-muted-foreground">/</span>
              <span className="text-muted-foreground">
                {version.subject.split("—")[0]?.trim() ?? "Template"}
              </span>
              <span className="text-muted-foreground">/</span>
              <span className="font-medium">
                Version {version.version_number} ({
                  version.status.charAt(0).toUpperCase() + version.status.slice(1)
                })
              </span>
            </div>
          </div>
          <div className="flex items-center gap-2.5">
            <div className="flex items-center gap-1 rounded-md border bg-background h-8 px-1">
              <button
                type="button"
                className={`px-2 h-6 rounded-sm font-mono text-[11px] font-medium transition-colors ${
                  activeLocale === "default"
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                }`}
                onClick={() => handleSwitchLocale("default")}
              >
                {version.default_locale}
              </button>

              {existingLocales.map((loc) => (
                <div key={loc.locale} className="relative group flex items-center">
                  <button
                    type="button"
                    className={`px-2 h-6 rounded-sm font-mono text-[11px] font-medium transition-colors ${
                      activeLocale === loc.locale
                        ? "bg-primary text-primary-foreground"
                        : "text-muted-foreground hover:text-foreground hover:bg-muted"
                    }`}
                    onClick={() => handleSwitchLocale(loc.locale)}
                  >
                    {loc.locale}
                  </button>
                  {isDraft && (
                    <button
                      type="button"
                      className="absolute -top-1.5 -right-1.5 hidden group-hover:flex items-center justify-center w-3.5 h-3.5 rounded-full bg-destructive text-destructive-foreground"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeleteLocale(loc.locale);
                      }}
                      title={`Remove ${loc.locale}`}
                    >
                      <X className="h-2 w-2" />
                    </button>
                  )}
                </div>
              ))}

              {isDraft && (
                <Popover open={addLocaleOpen} onOpenChange={setAddLocaleOpen}>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="h-6 w-6 flex items-center justify-center rounded-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    >
                      <Plus className="h-3.5 w-3.5" />
                    </button>
                  </PopoverTrigger>
                  <PopoverContent align="start" className="w-56 p-2">
                    <AddLocalePopover
                      existingLocales={[version.default_locale, ...existingLocales.map((l) => l.locale)]}
                      onAdd={handleAddLocale}
                      isAdding={saveLocaleMutation.isPending}
                    />
                  </PopoverContent>
                </Popover>
              )}
            </div>

            <div className="flex items-center gap-1 rounded-md border bg-background">
              <button
                className={`h-8 px-2.5 text-xs ${
                  editorMode === "visual"
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => setEditorMode("visual")}
                type="button"
              >
                <Paintbrush className="h-3.5 w-3.5" />
              </button>
              <button
                className={`h-8 px-2.5 text-xs ${
                  editorMode === "code"
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => setEditorMode("code")}
                type="button"
              >
                <MonitorStop className="h-3.5 w-3.5" />
              </button>
            </div>

            {isDraft && (
              <>
                <SaveStatusIndicator
                  status={autoSave.status}
                  lastSavedAt={autoSave.lastSavedAt}
                  error={autoSave.error}
                  onRetry={() => autoSave.save()}
                />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => autoSave.save()}
                  disabled={autoSave.status === "saving"}
                >
                  <Save className="h-4 w-4 mr-1.5" />
                  Save
                </Button>
              </>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowTestSend(true)}
            >
              <Send className="h-4 w-4 mr-1.5" />
              Send Test
            </Button>
            {isDraft && (
              <Button size="sm" onClick={() => setShowPublishConfirm(true)}>
                <Rocket className="h-4 w-4 mr-1.5" />
                Publish
              </Button>
            )}
          </div>
        </div>
      </div>

      <div ref={layoutSplitRef} className="flex flex-1 overflow-hidden">
        {/* Left: editor */}
        <div className="flex flex-col flex-1 border-r min-w-0 overflow-hidden">
          <div className="flex flex-1 min-h-0">
            {sidebarOpen ? (
            <div className={`shrink-0 ${DEFAULT_BLOCK_WIDTH} border-r overflow-auto bg-muted/20 flex flex-col`}>
              <div className="flex items-center justify-between p-3 pb-0">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Panel</span>
                <button
                  type="button"
                  className="p-0.5 rounded hover:bg-muted text-muted-foreground"
                  onClick={() => setSidebarOpen(false)}
                  title="Collapse sidebar"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                </button>
              </div>
              <div className="p-3 pt-2 overflow-auto flex-1">
              {/* === Blocks (collapsible, visual mode only) === */}
              {editorMode === "visual" && (
                <>
                  <button
                    type="button"
                    className="flex items-center gap-1 mb-2 w-full text-left"
                    onClick={() => setBlocksOpen((prev) => !prev)}
                  >
                    {blocksOpen ? (
                      <ChevronDown className="h-3 w-3 text-muted-foreground" />
                    ) : (
                      <ChevronRight className="h-3 w-3 text-muted-foreground" />
                    )}
                    <h4 className="text-xs font-semibold text-muted-foreground">{t("blocks.label")}</h4>
                  </button>
                  {blocksOpen && (
                    <div className="grid grid-cols-2 gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("text")}
                        className="h-8"
                      >
                        <Type className="h-3.5 w-3.5 mr-1" /> {t("blocks.text")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("button")}
                        className="h-8"
                      >
                        {t("blocks.button")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("image")}
                        className="h-8"
                      >
                        <ImageIcon className="h-3.5 w-3.5 mr-1" /> {t("blocks.image")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("divider")}
                        className="h-8"
                      >
                        <Minus className="h-3.5 w-3.5 mr-1" /> {t("blocks.divider")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("spacer")}
                        className="h-8"
                      >
                        <Grip className="h-3.5 w-3.5 mr-1" /> {t("blocks.spacer")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("banner")}
                        className="h-8"
                      >
                        <LayoutTemplate className="h-3.5 w-3.5 mr-1" /> {t("blocks.banner")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("video")}
                        className="h-8"
                      >
                        <Play className="h-3.5 w-3.5 mr-1" /> {t("blocks.video")}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!isDraft}
                        onClick={() => addBlock("list")}
                        className="h-8"
                      >
                        <List className="h-3.5 w-3.5 mr-1" /> {t("blocks.list")}
                      </Button>
                    </div>
                  )}
                </>
              )}

              {/* === Variables & Injectors (collapsible) === */}
              <button
                type="button"
                className="flex items-center gap-1 mt-4 mb-2 w-full text-left"
                onClick={() => setVariablesOpen((prev) => !prev)}
              >
                {variablesOpen ? (
                  <ChevronDown className="h-3 w-3 text-muted-foreground" />
                ) : (
                  <ChevronRight className="h-3 w-3 text-muted-foreground" />
                )}
                <h4 className="text-xs font-semibold text-muted-foreground">Variables</h4>
              </button>
              {variablesOpen && (
              <>
              <div className="space-y-2">
                {templateVariables.length > 0 ? (
                  templateVariables
                    .filter((item) => item.category === "event")
                    .map((item) => (
                      <button
                        type="button"
                        key={item.id}
                        className="w-full rounded-md border bg-background p-2 text-xs text-left disabled:opacity-60 disabled:cursor-not-allowed"
                        draggable={editorMode === "visual"}
                        onDragStart={editorMode === "visual" ? (event) => onVariableCardDragStart(event, item) : undefined}
                        onClick={() => appendTemplateVariableToBlock(selectedBlockId ?? "", item)}
                        disabled={!isDraft}
                      >
                        <div className="font-mono text-[11px] truncate">{item.label}</div>
                        <div className="text-muted-foreground text-[10px]">{item.hint}</div>
                      </button>
                    ))
                ) : (
                  <p className="text-xs text-muted-foreground">No event variables yet</p>
                )}
              </div>

              {/* === Injectors (grouped, searchable) === */}
              {injectorVariableTokens.length > 0 && (() => {
                const searchLower = injectorSearch.toLowerCase();
                const grouped = injectorVariableTokens.reduce<Record<string, TemplateVariable[]>>((acc, item) => {
                  const injectorName = item.label.split(".")[0];
                  if (!acc[injectorName]) acc[injectorName] = [];
                  acc[injectorName].push(item);
                  return acc;
                }, {});
                const filteredGroups = Object.entries(grouped)
                  .map(([name, items]) => {
                    if (!searchLower) return { name, items };
                    const filtered = items.filter(
                      (item) =>
                        item.label.toLowerCase().includes(searchLower) ||
                        item.token.toLowerCase().includes(searchLower)
                    );
                    if (filtered.length > 0) return { name, items: filtered };
                    if (name.toLowerCase().includes(searchLower)) return { name, items };
                    return null;
                  })
                  .filter(Boolean) as { name: string; items: TemplateVariable[] }[];

                return (
                  <>
                    <div className="relative mt-3 mb-2">
                      <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
                      <Input
                        value={injectorSearch}
                        onChange={(e) => setInjectorSearch(e.target.value)}
                        placeholder={t("searchPlaceholder")}
                        className="h-7 pl-7 text-xs"
                      />
                    </div>
                    <TooltipProvider>
                      <div className="space-y-1">
                        {filteredGroups.map(({ name, items }) => {
                          const isOpen = injectorSearch ? true : (injectorGroupsOpen[name] ?? false);
                          return (
                            <div key={name}>
                              <button
                                type="button"
                                className="flex items-center gap-1 w-full text-left py-1"
                                onClick={() =>
                                  setInjectorGroupsOpen((prev) => ({
                                    ...prev,
                                    [name]: !(prev[name] ?? false),
                                  }))
                                }
                              >
                                {isOpen ? (
                                  <ChevronDown className="h-3 w-3 text-muted-foreground shrink-0" />
                                ) : (
                                  <ChevronRight className="h-3 w-3 text-muted-foreground shrink-0" />
                                )}
                                <span className="font-mono text-[11px] font-medium truncate">{name}</span>
                                <span className="text-[10px] text-muted-foreground ml-auto shrink-0">{items.length}</span>
                              </button>
                              {isOpen && (
                                <div className="space-y-1.5 pl-4 mt-1">
                                  {items.map((item) => {
                                    const fieldName = item.label.split(".").slice(1).join(".");
                                    return (
                                      <Tooltip key={item.id}>
                                        <TooltipTrigger asChild>
                                          <button
                                            type="button"
                                            className="w-full rounded-md border bg-background p-2 text-xs text-left disabled:opacity-60 disabled:cursor-not-allowed"
                                            draggable={editorMode === "visual"}
                                            onDragStart={editorMode === "visual" ? (event) => onVariableCardDragStart(event, item) : undefined}
                                            onClick={() => appendTemplateVariableToBlock(selectedBlockId ?? "", item)}
                                            disabled={!isDraft}
                                          >
                                            <div className="font-mono text-[11px] truncate">{fieldName}</div>
                                          </button>
                                        </TooltipTrigger>
                                        <TooltipContent side="right">
                                          <p className="font-mono font-semibold text-[11px]">{item.label}</p>
                                          {item.hint && <p className="text-[10px] opacity-80">{item.hint}</p>}
                                        </TooltipContent>
                                      </Tooltip>
                                    );
                                  })}
                                </div>
                              )}
                            </div>
                          );
                        })}
                        {filteredGroups.length === 0 && (
                          <p className="text-xs text-muted-foreground py-1">{t("noResults")}</p>
                        )}
                      </div>
                    </TooltipProvider>
                  </>
                );
              })()}
              </>
              )}
              </div>
            </div>
            ) : (
            <div className="shrink-0 w-8 border-r bg-muted/20 flex flex-col items-center pt-3">
              <button
                type="button"
                className="p-0.5 rounded hover:bg-muted text-muted-foreground"
                onClick={() => setSidebarOpen(true)}
                title="Expand sidebar"
              >
                <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
            )}

            {editorMode === "visual" ? (
              <div className="flex-1 overflow-auto p-4">
                {builderDocument && builderDocument.blocks.length > 0 && (
                  <div className="flex justify-end gap-1 mb-2">
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"
                      onClick={expandAllBlocks}
                      title={t("expandAll")}
                    >
                      <ChevronsUpDown className="h-3 w-3" />
                      {t("expandAll")}
                    </button>
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"
                      onClick={collapseAllBlocks}
                      title={t("collapseAll")}
                    >
                      <ChevronsDownUp className="h-3 w-3" />
                      {t("collapseAll")}
                    </button>
                  </div>
                )}
                {builderDocument ? (
                  <div className="space-y-2">
                    {builderDocument.blocks.map((block, index) => {
                      const isActive = selectedBlockId === block.id;
                      const canReceiveVariable =
                        block.type === "text" || block.type === "button" || block.type === "banner";

                      return (
                        <div key={block.id} className="space-y-2">
                          <div
                            className={`h-3 rounded transition-colors ${
                              blockDropIndex === index && draggedBlockId
                                ? "bg-primary/70"
                                : "bg-transparent"
                            }`}
                            onDragOver={(event) =>
                              handleBlockDropZoneDragOver(event, index)
                            }
                            onDrop={(event) =>
                              handleBlockDropZoneDrop(event, index)
                            }
                            onDragLeave={() => {
                              if (blockDropIndex === index) {
                                setBlockDropIndex(null);
                              }
                            }}
                          />

                          <div
                            className={`rounded-md border p-3 ${
                              isActive ? "border-primary bg-primary/5" : "bg-background"
                            }`}
                            onDragOver={(ev) => {
                              if (!isDraft) return;
                              if (isBlockDragEvent(ev.dataTransfer)) {
                                ev.preventDefault();
                                ev.dataTransfer.dropEffect = "move";
                                const rect = ev.currentTarget.getBoundingClientRect();
                                const nextIndex =
                                  ev.clientY > rect.top + rect.height / 2
                                    ? index + 1
                                    : index;
                                setBlockDropIndex(nextIndex);
                                return;
                              }

                              if (!canReceiveVariable || !isVariableDragEvent(ev.dataTransfer))
                                return;
                              ev.preventDefault();
                              ev.dataTransfer.dropEffect = "copy";
                            }}
                            onDrop={(ev) => {
                              ev.preventDefault();
                              if (!isDraft) return;
                              if (isBlockDragEvent(ev.dataTransfer)) {
                                const rect = ev.currentTarget.getBoundingClientRect();
                                const nextIndex =
                                  ev.clientY > rect.top + rect.height / 2
                                    ? index + 1
                                    : index;
                                moveBlockToIndex(getDraggedBlockId(ev.dataTransfer) ?? "", nextIndex);
                                draggedBlockIdRef.current = null;
                                setDraggedBlockId(null);
                                setBlockDropIndex(null);
                                return;
                              }

                              if (!canReceiveVariable) return;
                              const variable = resolveVariableFromDrop(ev);
                              if (variable) {
                                appendTemplateVariableToBlock(block.id, variable);
                              }
                            }}
                            onClick={() => setSelectedBlockId(block.id)}
                          >
                            <div className="flex items-center justify-between mb-2">
                              <div className="flex items-center gap-1.5 text-xs text-muted-foreground min-w-0 flex-1">
                                <button
                                  type="button"
                                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded hover:bg-muted"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    toggleBlockCollapsed(block.id);
                                  }}
                                  title={collapsedBlocks[block.id] ? "Expand block" : "Collapse block"}
                                >
                                  {collapsedBlocks[block.id] ? (
                                    <ChevronRight className="h-3.5 w-3.5" />
                                  ) : (
                                    <ChevronDown className="h-3.5 w-3.5" />
                                  )}
                                </button>
                                <button
                                  type="button"
                                  className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded hover:bg-muted cursor-grab active:cursor-grabbing"
                                  draggable={isDraft}
                                  onDragStart={(event) =>
                                    onBlockHandleDragStart(event, block.id)
                                  }
                                  onDragEnd={handleBlockDragEnd}
                                  onClick={(event) => event.stopPropagation()}
                                  title={t("reorderBlock")}
                                >
                                  <GripVertical className="h-3.5 w-3.5" />
                                </button>
                                <input
                                  key={block.id}
                                  type="text"
                                  className="bg-transparent border-none outline-none text-xs text-muted-foreground min-w-0 flex-1 px-0.5 rounded hover:bg-muted/50 focus:bg-muted/50 focus:text-foreground"
                                  defaultValue={block.label || defaultBlockLabel[block.type]}
                                  onBlur={(ev) => updateBlockLabel(block.id, ev.target.value)}
                                  onKeyDown={(ev) => {
                                    if (ev.key === "Enter") ev.currentTarget.blur();
                                    if (ev.key === "Escape") {
                                      ev.currentTarget.value = block.label || defaultBlockLabel[block.type];
                                      ev.currentTarget.blur();
                                    }
                                  }}
                                  onClick={(ev) => ev.stopPropagation()}
                                  readOnly={!isDraft}
                                />
                              </div>
                              {isDraft ? (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  className="h-7 px-2 shrink-0"
                                  onClick={() => removeBlock(block.id)}
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              ) : null}
                            </div>

                            {!collapsedBlocks[block.id] && (<>
                            {block.type === "text" && (
                              <TextBlockEditor
                                ref={(handle) => {
                                  if (handle) {
                                    textBlockEditorRefs.current[block.id] = handle;
                                  } else {
                                    delete textBlockEditorRefs.current[block.id];
                                  }
                                }}
                                content={block.content}
                                onChange={(html, newAlign) => updateTextBlock(block.id, html, newAlign)}
                                align={block.align}
                                disabled={!isDraft}
                                onFocus={() => setSelectedBlockId(block.id)}
                              />
                            )}

                            {block.type === "button" && (
                              <>
                                <Label className="text-xs">{t("blockContent")}</Label>
                                <div className="mt-1 rounded-md border border-input bg-background px-2 py-1.5 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
                                  <div
                                    ref={(node) => {
                                      if (node) {
                                        blockEditorRefs.current[block.id] = node;
                                      } else {
                                        delete blockEditorRefs.current[block.id];
                                      }
                                    }}
                                    contentEditable={isDraft}
                                    suppressContentEditableWarning
                                    className="min-h-6 w-full whitespace-pre-wrap break-words text-sm font-mono outline-none empty:before:content-[attr(data-placeholder)] empty:before:text-muted-foreground"
                                    data-placeholder={t("textPlaceholder")}
                                    onInput={(event) =>
                                      handleBlockEditorInput(block.id, event.currentTarget)
                                    }
                                    onKeyDown={(event) =>
                                      handleBlockEditorKeyDown(block.id, event)
                                    }
                                    onClick={handleBlockEditorTokenClick}
                                    onFocus={() => setSelectedBlockId(block.id)}
                                    onCopy={(event) =>
                                      handleBlockEditorCopyOrCut(block.id, event, "copy")
                                    }
                                    onCut={(event) =>
                                      handleBlockEditorCopyOrCut(block.id, event, "cut")
                                    }
                                    onPaste={(event) => handleBlockEditorPaste(block.id, event)}
                                    onDragOver={(event) =>
                                      handleBlockEditorDragOver(block.id, event)
                                    }
                                    onDrop={(event) => handleBlockEditorDrop(block.id, event)}
                                  />
                                </div>
                              </>
                            )}

                            {block.type === "button" ? (
                              <div className="mt-2">
                                <Label className="text-xs">URL</Label>
                                <Input
                                  value={block.href}
                                  className="h-8 mt-1"
                                  onChange={(ev) =>
                                    updateButtonHref(block.id, ev.target.value)
                                  }
                                  readOnly={!isDraft}
                                />
                              </div>
                            ) : null}

                            {block.type === "image" ? (
                              <>
                                <Label className="text-xs">Source</Label>
                                <Input
                                  value={block.src}
                                  className="h-8 mt-1"
                                  onChange={(ev) =>
                                    updateImageBlock(block.id, "src", ev.target.value)
                                  }
                                  readOnly={!isDraft}
                                />
                                <Label className="text-xs mt-2">Alt</Label>
                                <Input
                                  value={block.alt || ""}
                                  className="h-8 mt-1"
                                  onChange={(ev) =>
                                    updateImageBlock(block.id, "alt", ev.target.value)
                                  }
                                  readOnly={!isDraft}
                                />
                                <Label className="text-xs mt-2">Width</Label>
                                <Input
                                  value={block.width || ""}
                                  className="h-8 mt-1"
                                  onChange={(ev) =>
                                    updateImageBlock(block.id, "width", ev.target.value)
                                  }
                                  readOnly={!isDraft}
                                />
                              </>
                            ) : null}

                            {block.type === "spacer" ? (
                              <>
                                <Label className="text-xs">{t("blockHeight")}</Label>
                                <Input
                                  type="number"
                                  min={0}
                                  step={1}
                                  value={block.height}
                                  className="h-8 mt-1"
                                  onChange={(ev) =>
                                    updateSpacer(
                                      block.id,
                                      Number.parseInt(ev.target.value, 10)
                                    )
                                  }
                                  readOnly={!isDraft}
                                />
                              </>
                            ) : null}

                            {block.type === "divider" ? (
                              <div className="text-xs text-muted-foreground">
                                {t("blockNoProperties")}
                              </div>
                            ) : null}

                            {block.type === "banner" && (
                              <div className="space-y-2">
                                <div>
                                  <Label className="text-xs">Background Image URL</Label>
                                  <Input
                                    value={block.backgroundUrl}
                                    className="h-8 mt-1"
                                    placeholder="https://example.com/hero.jpg"
                                    onChange={(ev) => updateBannerBlock(block.id, "backgroundUrl", ev.target.value)}
                                    readOnly={!isDraft}
                                  />
                                </div>
                                <div className="flex gap-2">
                                  <div className="flex-1">
                                    <Label className="text-xs">Background Color</Label>
                                    <Input
                                      type="color"
                                      value={block.backgroundColor}
                                      className="h-8 mt-1 w-16"
                                      onChange={(ev) => updateBannerBlock(block.id, "backgroundColor", ev.target.value)}
                                      readOnly={!isDraft}
                                    />
                                  </div>
                                  <div className="flex-1">
                                    <Label className="text-xs">Mode</Label>
                                    <select
                                      value={block.mode}
                                      onChange={(ev) => updateBannerBlock(block.id, "mode", ev.target.value)}
                                      disabled={!isDraft}
                                      className="h-8 mt-1 w-full rounded-md border border-input bg-background px-2 text-sm"
                                    >
                                      <option value="fluid-height">Fluid</option>
                                      <option value="fixed-height">Fixed</option>
                                    </select>
                                  </div>
                                </div>
                                {block.mode === "fixed-height" && (
                                  <div>
                                    <Label className="text-xs">Height (px)</Label>
                                    <Input
                                      type="number"
                                      min={100}
                                      value={block.height}
                                      className="h-8 mt-1"
                                      onChange={(ev) => updateBannerBlock(block.id, "height", Number.parseInt(ev.target.value, 10) || 400)}
                                      readOnly={!isDraft}
                                    />
                                  </div>
                                )}
                                <div>
                                  <Label className="text-xs">Overlay Text</Label>
                                  <div className="mt-1 rounded-md border border-input bg-background px-2 py-1.5 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
                                    <div
                                      ref={(node) => {
                                        if (node) {
                                          blockEditorRefs.current[block.id] = node;
                                        } else {
                                          delete blockEditorRefs.current[block.id];
                                        }
                                      }}
                                      contentEditable={isDraft}
                                      suppressContentEditableWarning
                                      className="min-h-6 w-full whitespace-pre-wrap break-words text-sm font-mono outline-none empty:before:content-[attr(data-placeholder)] empty:before:text-muted-foreground"
                                      data-placeholder="Headline text..."
                                      onInput={(event) =>
                                        handleBlockEditorInput(block.id, event.currentTarget)
                                      }
                                      onKeyDown={(event) =>
                                        handleBlockEditorKeyDown(block.id, event)
                                      }
                                      onClick={handleBlockEditorTokenClick}
                                      onFocus={() => setSelectedBlockId(block.id)}
                                      onCopy={(event) =>
                                        handleBlockEditorCopyOrCut(block.id, event, "copy")
                                      }
                                      onCut={(event) =>
                                        handleBlockEditorCopyOrCut(block.id, event, "cut")
                                      }
                                      onPaste={(event) => handleBlockEditorPaste(block.id, event)}
                                      onDragOver={(event) =>
                                        handleBlockEditorDragOver(block.id, event)
                                      }
                                      onDrop={(event) => handleBlockEditorDrop(block.id, event)}
                                    />
                                  </div>
                                </div>
                                <div>
                                  <Label className="text-xs">Button Text (optional)</Label>
                                  <Input
                                    value={block.buttonText}
                                    className="h-8 mt-1"
                                    placeholder="Leave empty for no button"
                                    onChange={(ev) => updateBannerBlock(block.id, "buttonText", ev.target.value)}
                                    readOnly={!isDraft}
                                  />
                                </div>
                                {block.buttonText && (
                                  <div className="flex gap-2">
                                    <div className="flex-1">
                                      <Label className="text-xs">Button URL</Label>
                                      <Input
                                        value={block.buttonHref}
                                        className="h-8 mt-1"
                                        onChange={(ev) => updateBannerBlock(block.id, "buttonHref", ev.target.value)}
                                        readOnly={!isDraft}
                                      />
                                    </div>
                                    <div>
                                      <Label className="text-xs">Button Color</Label>
                                      <Input
                                        type="color"
                                        value={block.buttonColor}
                                        className="h-8 mt-1 w-16"
                                        onChange={(ev) => updateBannerBlock(block.id, "buttonColor", ev.target.value)}
                                        readOnly={!isDraft}
                                      />
                                    </div>
                                  </div>
                                )}
                                <div className="flex gap-2">
                                  <div className="flex-1">
                                    <Label className="text-xs">V. Align</Label>
                                    <select
                                      value={block.verticalAlign}
                                      onChange={(ev) => updateBannerBlock(block.id, "verticalAlign", ev.target.value)}
                                      disabled={!isDraft}
                                      className="h-8 mt-1 w-full rounded-md border border-input bg-background px-2 text-sm"
                                    >
                                      <option value="top">Top</option>
                                      <option value="middle">Middle</option>
                                      <option value="bottom">Bottom</option>
                                    </select>
                                  </div>
                                  <div className="flex-1">
                                    <Label className="text-xs">Padding (px)</Label>
                                    <Input
                                      type="number"
                                      min={0}
                                      value={block.padding}
                                      className="h-8 mt-1"
                                      onChange={(ev) => updateBannerBlock(block.id, "padding", Number.parseInt(ev.target.value, 10) || 0)}
                                      readOnly={!isDraft}
                                    />
                                  </div>
                                </div>
                              </div>
                            )}

                            {block.type === "video" && (
                              <div className="space-y-2">
                                <div>
                                  <Label className="text-xs">Video URL</Label>
                                  <Input
                                    value={block.videoUrl}
                                    className="h-8 mt-1"
                                    placeholder="https://youtube.com/watch?v=..."
                                    onChange={(ev) => {
                                      if (!builderDocument) return;
                                      const url = ev.target.value;
                                      const thumb = extractVideoThumbnail(url);
                                      updateBuilderDocument({
                                        ...builderDocument,
                                        blocks: builderDocument.blocks.map((b) => {
                                          if (b.id !== block.id || b.type !== "video") return b;
                                          return {
                                            ...b,
                                            videoUrl: url,
                                            ...(thumb ? { thumbnailUrl: thumb } : {}),
                                          };
                                        }),
                                      });
                                    }}
                                    readOnly={!isDraft}
                                  />
                                </div>
                                <div>
                                  <Label className="text-xs">Thumbnail URL</Label>
                                  <Input
                                    value={block.thumbnailUrl}
                                    className="h-8 mt-1"
                                    placeholder="https://img.youtube.com/vi/ID/maxresdefault.jpg"
                                    onChange={(ev) => updateVideoBlock(block.id, "thumbnailUrl", ev.target.value)}
                                    readOnly={!isDraft}
                                  />
                                </div>
                                {block.thumbnailUrl && (
                                  <div className="mt-1 rounded border overflow-hidden relative">
                                    {/* eslint-disable-next-line @next/next/no-img-element -- editor accepts arbitrary remote URLs for thumbnail preview. */}
                                    <img
                                      src={block.thumbnailUrl}
                                      alt={block.alt || "Video thumbnail"}
                                      className="w-full h-auto"
                                    />
                                    <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                                      <div className="w-12 h-12 rounded-full bg-black/60 flex items-center justify-center">
                                        <Play className="h-5 w-5 text-white ml-0.5" />
                                      </div>
                                    </div>
                                  </div>
                                )}
                                <div>
                                  <Label className="text-xs">Alt Text</Label>
                                  <Input
                                    value={block.alt}
                                    className="h-8 mt-1"
                                    onChange={(ev) => updateVideoBlock(block.id, "alt", ev.target.value)}
                                    readOnly={!isDraft}
                                  />
                                </div>
                                <div>
                                  <Label className="text-xs">Width</Label>
                                  <Input
                                    value={block.width}
                                    className="h-8 mt-1"
                                    placeholder="100%"
                                    onChange={(ev) => updateVideoBlock(block.id, "width", ev.target.value)}
                                    readOnly={!isDraft}
                                  />
                                </div>
                              </div>
                            )}

                            {block.type === "list" && (
                              <div className="space-y-2">
                                <div>
                                  <Label className="text-xs">{t("blockListType")}</Label>
                                  <select
                                    value={block.listType}
                                    onChange={(ev) => updateListBlock(block.id, "listType", ev.target.value)}
                                    disabled={!isDraft}
                                    className="h-8 mt-1 w-full rounded-md border border-input bg-background px-2 text-sm"
                                  >
                                    <option value="bullet">Bullets</option>
                                    <option value="number">Numbers (1, 2, 3)</option>
                                    <option value="letter-upper">Letters (A, B, C)</option>
                                    <option value="letter-lower">Letters (a, b, c)</option>
                                    <option value="roman">Roman (I, II, III)</option>
                                  </select>
                                </div>
                                <div>
                                  <Label className="text-xs">Items</Label>
                                  <div className="mt-1">
                                    {(function renderItems(items: ListItem[], depth: number): React.ReactNode {
                                      return items.map((item, idx) => {
                                        const isDragging = draggedListItemId === item.id;
                                        const isDropBefore = listItemDropTarget?.itemId === item.id && listItemDropTarget.position === "before";
                                        const isDropAfter = listItemDropTarget?.itemId === item.id && listItemDropTarget.position === "after";

                                        function handleListDragOver(ev: React.DragEvent<HTMLDivElement>) {
                                          if (!ev.dataTransfer.types.includes(LIST_ITEM_DND_MIME)) return;
                                          ev.preventDefault();
                                          ev.stopPropagation();
                                          ev.dataTransfer.dropEffect = "move";
                                          const rect = ev.currentTarget.getBoundingClientRect();
                                          const pos = ev.clientY < rect.top + rect.height / 2 ? "before" : "after";
                                          setListItemDropTarget({ itemId: item.id, position: pos });
                                        }

                                        function handleListDrop(ev: React.DragEvent<HTMLDivElement>) {
                                          ev.preventDefault();
                                          ev.stopPropagation();
                                          const srcId = ev.dataTransfer.getData(LIST_ITEM_DND_MIME);
                                          const pos = listItemDropTarget?.itemId === item.id ? listItemDropTarget.position : "after";
                                          if (srcId && srcId !== item.id) {
                                            moveListItem(block.id, srcId, item.id, pos);
                                          }
                                          setDraggedListItemId(null);
                                          setListItemDropTarget(null);
                                        }

                                        return (
                                        <div key={item.id} style={{ paddingLeft: `${depth * 16}px` }}>
                                          <div
                                            className={`relative flex items-center gap-1 rounded px-0.5 transition-opacity ${isDragging ? "opacity-30" : ""}`}
                                            onDragOver={handleListDragOver}
                                            onDragLeave={() => {
                                              if (listItemDropTarget?.itemId === item.id) setListItemDropTarget(null);
                                            }}
                                            onDrop={handleListDrop}
                                          >
                                            {/* Drop indicator line */}
                                            {isDropBefore && (
                                              <div className="absolute -top-px left-0 right-0 h-0.5 bg-primary rounded pointer-events-none" />
                                            )}
                                            {isDropAfter && (
                                              <div className="absolute -bottom-px left-0 right-0 h-0.5 bg-primary rounded pointer-events-none" />
                                            )}
                                            {isDraft && (
                                              <button
                                                type="button"
                                                className="h-5 w-4 inline-flex items-center justify-center shrink-0 cursor-grab active:cursor-grabbing text-muted-foreground hover:text-foreground"
                                                draggable
                                                onDragStart={(ev) => {
                                                  ev.stopPropagation();
                                                  ev.dataTransfer.setData(LIST_ITEM_DND_MIME, item.id);
                                                  ev.dataTransfer.effectAllowed = "move";
                                                  setDraggedListItemId(item.id);
                                                }}
                                                onDragEnd={() => {
                                                  setDraggedListItemId(null);
                                                  setListItemDropTarget(null);
                                                }}
                                              >
                                                <GripVertical className="h-3 w-3" />
                                              </button>
                                            )}
                                            <span className="text-xs text-muted-foreground w-5 shrink-0 text-right">
                                              {block.listType === "bullet" ? "\u2022" : `${idx + 1}.`}
                                            </span>
                                            <Input
                                              value={renderSegmentsToText(item.segments)}
                                              className="h-7 text-xs flex-1"
                                              onChange={(ev) =>
                                                updateListItemSegments(block.id, item.id, ev.target.value)
                                              }
                                              readOnly={!isDraft}
                                            />
                                            {isDraft && (
                                              <div className="flex items-center gap-0.5 shrink-0">
                                                {idx > 0 && depth < 2 && (
                                                  <button
                                                    type="button"
                                                    className="h-5 w-5 inline-flex items-center justify-center rounded hover:bg-muted text-muted-foreground"
                                                    onClick={() => indentListItem(block.id, item.id)}
                                                    title="Indent"
                                                  >
                                                    <ChevronRight className="h-3 w-3" />
                                                  </button>
                                                )}
                                                {depth > 0 && (
                                                  <button
                                                    type="button"
                                                    className="h-5 w-5 inline-flex items-center justify-center rounded hover:bg-muted text-muted-foreground"
                                                    onClick={() => outdentListItem(block.id, item.id)}
                                                    title="Outdent"
                                                  >
                                                    <ChevronLeft className="h-3 w-3" />
                                                  </button>
                                                )}
                                                <button
                                                  type="button"
                                                  className="h-5 w-5 inline-flex items-center justify-center rounded hover:bg-muted text-muted-foreground"
                                                  onClick={() => addListItem(block.id, item.id)}
                                                  title="Add item below"
                                                >
                                                  <Plus className="h-3 w-3" />
                                                </button>
                                                <button
                                                  type="button"
                                                  className="h-5 w-5 inline-flex items-center justify-center rounded hover:bg-muted text-destructive"
                                                  onClick={() => removeListItem(block.id, item.id)}
                                                  title="Remove item"
                                                >
                                                  <Trash2 className="h-3 w-3" />
                                                </button>
                                              </div>
                                            )}
                                          </div>
                                          {item.children.length > 0 && renderItems(item.children, depth + 1)}
                                        </div>
                                        );
                                      });
                                    })(block.items, 0)}
                                  </div>
                                  {isDraft && (
                                    <Button
                                      size="sm"
                                      variant="outline"
                                      className="h-7 mt-2 w-full text-xs"
                                      onClick={() => addListItem(block.id)}
                                    >
                                      <Plus className="h-3 w-3 mr-1" /> Add Item
                                    </Button>
                                  )}
                                </div>
                              </div>
                            )}
                            </>)}
                          </div>
                        </div>
                      );
                    })}

                    <div
                      className={`h-3 rounded transition-colors ${
                        blockDropIndex === builderDocument.blocks.length &&
                        draggedBlockId
                          ? "bg-primary/70"
                          : "bg-transparent"
                      }`}
                      onDragOver={(event) =>
                        handleBlockDropZoneDragOver(
                          event,
                          builderDocument.blocks.length
                        )
                      }
                      onDrop={(event) =>
                        handleBlockDropZoneDrop(event, builderDocument.blocks.length)
                      }
                      onDragLeave={() => {
                        if (blockDropIndex === builderDocument.blocks.length) {
                          setBlockDropIndex(null);
                        }
                      }}
                    />
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="relative flex-1 min-w-0">
                <div className="absolute inset-0">
                  <MonacoEditorWrapper
                    value={codeMjml}
                    onChange={handleCodeChange}
                    readOnly={!isDraft}
                  />
                </div>
              </div>
            )}
          </div>

          <div className="border-t bg-card p-4 shrink-0">
            <button
              type="button"
              className="flex items-center gap-1 w-full text-left mb-1"
              onClick={() => setMetadataOpen((prev) => !prev)}
            >
              {metadataOpen ? (
                <ChevronDown className="h-3 w-3 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-3 w-3 text-muted-foreground" />
              )}
              <h4 className="text-sm font-semibold">Metadata</h4>
            </button>
            {metadataOpen && (
            <>
            <p className="mb-3 text-xs text-muted-foreground">
              Used for email headers and inbox preview in real sends.
            </p>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Subject</Label>
                <Input
                  {...register("subject")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
                {errors.subject && (
                  <span className="text-xs text-destructive">
                    {errors.subject.message}
                  </span>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Preview Text</Label>
                <Input
                  {...register("preview_text")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">From Name</Label>
                <Input
                  {...register("from_name")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
                {errors.from_name && (
                  <span className="text-xs text-destructive">
                    {errors.from_name.message}
                  </span>
                )}
              </div>
              {activeLocale === "default" && (
                <div className="flex flex-col gap-1.5">
                  <Label className="text-xs font-medium">Reply-To</Label>
                  <Input
                    {...register("reply_to")}
                    className="h-8 text-sm"
                    readOnly={!isDraft}
                  />
                </div>
              )}
            </div>
            </>
            )}
          </div>
        </div>

        {/* Right panel */}
        <div
          ref={previewPanelWrapRef}
          className="relative flex min-w-0"
          style={{
            flex:
              previewSplitMode === "ratio"
                ? "1 1 0%"
                : `0 1 ${previewPanelWidthPx + PANEL_RESIZER_WIDTH}px`,
          }}
        >
          <button
            type="button"
            aria-label="Resize preview panel"
            className="w-3 cursor-col-resize bg-muted/40 hover:bg-muted touch-none shrink-0 flex items-center justify-center"
            onMouseDown={startPreviewResize}
          >
            <span className="h-8 w-px bg-border" />
          </button>

          <div className="flex flex-col flex-1 min-w-0">
            <div className="flex items-center justify-between h-10 px-4 border-b bg-card">
              <span className="text-sm font-semibold">Preview</span>
            </div>
            <div className="flex-1 overflow-hidden bg-surface p-6">
              <div
                ref={previewStageCallbackRef}
                className="flex h-full w-full items-start justify-center overflow-hidden"
              >
                {previewFrameUrl ? (
                  <div
                    className="relative overflow-hidden rounded-md border border-border/60 bg-card"
                    style={{
                      width: `${previewScaledWidth}px`,
                      height: `${previewStageSize.height > 0 ? previewStageSize.height : previewScaledHeight}px`,
                    }}
                  >
                    <iframe
                      ref={previewIframeRef}
                      src={previewFrameUrl}
                      onLoad={handlePreviewIframeLoad}
                      className="pointer-events-none border-0 bg-card"
                      style={{
                        width: `${previewNaturalWidth}px`,
                        height: `${previewNaturalHeight}px`,
                        transform: `scale(${previewScale})`,
                        transformOrigin: "top left",
                      }}
                      sandbox="allow-same-origin allow-scripts"
                      title="Email Preview"
                    />
                  </div>
                ) : (
                  <div
                    className="flex items-center justify-center rounded-md border bg-card text-sm text-muted-foreground"
                    style={{
                      width: `${previewScaledWidth}px`,
                      height: `${previewStageSize.height > 0 ? previewStageSize.height : previewScaledHeight}px`,
                    }}
                  >
                    {previewMutation.isPending
                      ? "Generating preview..."
                      : "Write MJML or build visually to see preview"}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={showPublishConfirm}
        onOpenChange={setShowPublishConfirm}
        title="Publish Version"
        description={`Publishing version ${version.version_number} will make it the active template. The current published version will be archived. This action cannot be undone.`}
        confirmLabel="Publish"
        variant="default"
        onConfirm={handlePublish}
        loading={publishMutation.isPending}
      />

      <TestSendModal
        open={showTestSend}
        onOpenChange={setShowTestSend}
        scopedPath={scopedPath}
        templateId={templateId}
        locale={activeLocale === "default" ? undefined : activeLocale}
      />
    </div>
  );
}

type MonacoEditorComponent = ComponentType<Record<string, unknown>>;

/** Wrapper for lazy-loaded Monaco editor */
function MonacoEditorWrapper({
  value,
  onChange,
  readOnly,
}: {
  value: string;
  onChange: (value: string) => void;
  readOnly: boolean;
}) {
  const [Editor, setEditor] = useState<MonacoEditorComponent | null>(null);
  const { resolvedTheme } = useTheme();

  useEffect(() => {
    import("@monaco-editor/react").then((mod) => {
      setEditor(() => mod.default as MonacoEditorComponent);
    });
  }, []);

  if (!Editor) {
    return (
      <div className="flex items-center justify-center h-full bg-muted/20">
        <span className="text-muted-foreground text-sm">Loading editor...</span>
      </div>
    );
  }

  return (
    <Editor
      value={value}
      onChange={(val: string | undefined) => onChange(val ?? "")}
      language="xml"
      theme={resolvedTheme === "dark" ? "vs-dark" : "light"}
      options={{
        minimap: { enabled: false },
        fontSize: 13,
        fontFamily: "'IBM Plex Mono', monospace",
        lineNumbers: "on",
        scrollBeyondLastLine: false,
        wordWrap: "on",
        readOnly,
        padding: { top: 16 },
      }}
      className="h-full"
    />
  );
}

function AddLocalePopover({
  existingLocales,
  onAdd,
  isAdding,
}: {
  existingLocales: string[];
  onAdd: (locale: string) => void;
  isAdding: boolean;
}) {
  const [input, setInput] = useState("");
  const COMMON_LOCALES = [
    "es", "fr", "de", "pt", "pt-BR", "it", "nl", "pl", "ru", "ja", "zh",
    "zh-TW", "ko", "ar", "tr", "sv", "da", "fi", "nb", "cs", "ro", "hu",
  ];
  const available = COMMON_LOCALES.filter((l) => !existingLocales.includes(l));
  const filtered = input.trim()
    ? available.filter((l) => l.toLowerCase().startsWith(input.toLowerCase()))
    : available;

  return (
    <div className="flex flex-col gap-1.5">
      <Input
        placeholder="e.g. es, pt-BR"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        className="h-7 text-xs font-mono"
        onKeyDown={(e) => {
          if (e.key === "Enter" && input.trim() && !existingLocales.includes(input.trim())) {
            onAdd(input.trim());
          }
        }}
        autoFocus
      />
      <div className="flex flex-wrap gap-1 max-h-40 overflow-y-auto">
        {filtered.map((l) => (
          <button
            key={l}
            type="button"
            disabled={isAdding}
            className="px-1.5 py-0.5 rounded border text-[11px] font-mono text-muted-foreground hover:bg-primary hover:text-primary-foreground transition-colors disabled:opacity-50"
            onClick={() => onAdd(l)}
          >
            {l}
          </button>
        ))}
      </div>
      {input.trim() && !existingLocales.includes(input.trim()) && !COMMON_LOCALES.includes(input.trim()) && (
        <button
          type="button"
          disabled={isAdding}
          onClick={() => onAdd(input.trim())}
          className="text-xs text-muted-foreground hover:text-foreground text-left"
        >
          Add &quot;{input.trim()}&quot; as custom locale
        </button>
      )}
    </div>
  );
}

function MjmlEditorSkeleton() {
  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between h-14 px-6 border-b bg-card">
        <Skeleton className="h-5 w-64" />
        <div className="flex gap-2">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-8 w-20" />
        </div>
      </div>
      <div className="flex flex-1">
        <div className="flex-1 bg-background" />
        <div className="w-[480px] border-l bg-surface" />
      </div>
    </div>
  );
}
