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
  MousePointer,
  Image as ImageIcon,
  Minus,
  Grip,
  Plus,
  ChevronDown,
  ChevronRight,
  Search,
} from "lucide-react";
import { useScope, useScopedPath } from "@/hooks/use-scope";
import {
  useTemplateVersion,
  useSaveTemplateVersion,
  usePublishVersion,
  usePreviewMjml,
} from "@/hooks/use-template-version";
import { useTemplateType } from "@/hooks/use-template-types";
import { useInjectorList } from "@/hooks/use-injectors";
import { useApi } from "@/hooks/use-api";
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
import { toast } from "sonner";
import type { CreateTemplateVersionRequest } from "@/types/templates";
import type { InjectorDefinition, InjectorWithValues } from "@/types/injectors";

const metadataSchema = z.object({
  subject: z.string().min(1, { message: "Subject is required" }),
  preview_text: z.string().optional(),
  from_name: z.string().min(1, { message: "From name is required" }),
  reply_to: z.string().optional(),
});

type MetadataForm = z.infer<typeof metadataSchema>;

type BuilderBlockType = "text" | "button" | "image" | "divider" | "spacer";

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
  type: "text";
  segments: BuilderSegment[];
  align: "left" | "center" | "right";
};

type BuilderButtonBlock = {
  id: string;
  type: "button";
  segments: BuilderSegment[];
  href: string;
  align: "left" | "center" | "right";
};

type BuilderImageBlock = {
  id: string;
  type: "image";
  src: string;
  alt?: string;
  width?: string;
  align: "left" | "center" | "right";
};

type BuilderDividerBlock = {
  id: string;
  type: "divider";
};

type BuilderSpacerBlock = {
  id: string;
  type: "spacer";
  height: number;
};

type BuilderBlock =
  | BuilderTextBlock
  | BuilderButtonBlock
  | BuilderImageBlock
  | BuilderDividerBlock
  | BuilderSpacerBlock;

type BuilderDocument = {
  version: number;
  blocks: BuilderBlock[];
};

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
        segments: [createTextSegment("Drag content here")],
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
      const align = (() => {
        const candidate = (item as { align?: unknown }).align;
        return candidate === "left" || candidate === "center" || candidate === "right"
          ? candidate
          : "left";
      })();

      if (item.type === "text") {
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
                };
              }
              return null;
            })
            .filter((segment): segment is BuilderSegment => segment !== null);
          const uniqueSegments = ensureUniqueSegmentIds(sanitized);
          return {
            id,
            type: "text",
            segments: uniqueSegments.length ? uniqueSegments : [createTextSegment("")],
            align,
          };
        }

        const legacyContent =
          typeof (item as { content?: unknown }).content === "string"
            ? ((item as { content?: unknown }).content as string)
            : "";
        return {
          id,
          type: "text",
          segments: ensureUniqueSegmentIds(parseContentToSegments(legacyContent)),
          align,
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
        return { id, type: "image", src, alt, width, align };
      }
      if (item.type === "divider") {
        return { id, type: "divider" };
      }
      if (item.type === "spacer") {
        const height = normalizeSpacerHeight(
          (item as { height?: unknown }).height,
          20
        );
        return { id, type: "spacer", height };
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

  try {
    const parser = new DOMParser();
    const documentRef = parser.parseFromString(rawHtml, "text/html");

    documentRef
      .querySelectorAll(
        "script, noscript, iframe, object, embed, link[rel='preload'][as='script']"
      )
      .forEach((element) => element.remove());

    documentRef.querySelectorAll("*").forEach((element) => {
      for (const attribute of Array.from(element.attributes)) {
        const attrName = attribute.name.toLowerCase();
        const attrValue = attribute.value.trim().toLowerCase();

        if (attrName.startsWith("on")) {
          element.removeAttribute(attribute.name);
          continue;
        }

        if (
          (attrName === "src" || attrName === "href" || attrName === "xlink:href") &&
          attrValue.startsWith("javascript:")
        ) {
          element.removeAttribute(attribute.name);
        }
      }
    });

    return `<!doctype html>\n${documentRef.documentElement.outerHTML}`;
  } catch {
    return rawHtml.replace(
      /<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi,
      ""
    );
  }
}

function getPreviewScale(contentSize: PreviewDocumentSize, stage: PreviewStageSize) {
  const safeWidth = Math.max(1, contentSize.width);
  const safeHeight = Math.max(1, contentSize.height);
  if (stage.width <= 0 || stage.height <= 0) {
    return 1;
  }
  return Math.max(
    MIN_PREVIEW_SCALE,
    Math.min(1, stage.width / safeWidth, stage.height / safeHeight)
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

function parseMjmlAlign(value: string | null): "left" | "center" | "right" {
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

    const column = xmlDoc.getElementsByTagName("mj-column")[0];
    if (!column) {
      return null;
    }

    const blocks: BuilderBlock[] = [];
    for (const child of Array.from(column.children)) {
      const tag = child.tagName.toLowerCase();

      if (tag === "mj-text") {
        blocks.push({
          id: nowId(),
          type: "text",
          segments: ensureUniqueSegmentIds(
            parseContentToSegments(stripMjmlInlineTags(child.innerHTML ?? ""))
          ),
          align: parseMjmlAlign(child.getAttribute("align")),
        });
        continue;
      }

      if (tag === "mj-button") {
        const content = stripMjmlInlineTags(child.innerHTML ?? "");
        blocks.push({
          id: nowId(),
          type: "button",
          segments: ensureUniqueSegmentIds(parseContentToSegments(content || "Button")),
          href: child.getAttribute("href") || "#",
          align: parseMjmlAlign(child.getAttribute("align")),
        });
        continue;
      }

      if (tag === "mj-image") {
        blocks.push({
          id: nowId(),
          type: "image",
          src: child.getAttribute("src") || "",
          alt: child.getAttribute("alt") || undefined,
          width: child.getAttribute("width") || undefined,
          align: parseMjmlAlign(child.getAttribute("align")),
        });
        continue;
      }

      if (tag === "mj-divider") {
        blocks.push({
          id: nowId(),
          type: "divider",
        });
        continue;
      }

      if (tag === "mj-spacer") {
        blocks.push({
          id: nowId(),
          type: "spacer",
          height: normalizeSpacerHeight(child.getAttribute("height"), 20),
        });
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

function buildTemplateMjml(document: BuilderDocument) {
  const blocks = document.blocks
      .map((block) => {
      switch (block.type) {
        case "text":
          return `<mj-text>${renderSegmentsToText(block.segments).trim() || " "}</mj-text>`;
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
        default:
          return "";
      }
    })
    .join("");

  return `<mjml>\n  <mj-body>\n    <mj-section>\n      <mj-column>${blocks}\n      </mj-column>\n    </mj-section>\n  </mj-body>\n</mjml>`;
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
  const [blocksOpen, setBlocksOpen] = useState(true);
  const [injectorGroupsOpen, setInjectorGroupsOpen] = useState<Record<string, boolean>>({});
  const [injectorSearch, setInjectorSearch] = useState("");

  const [editorMode, setEditorMode] = useState<EditorMode>("visual");
  const [previewHtml, setPreviewHtml] = useState("");
  const [previewFrameUrl, setPreviewFrameUrl] = useState("");
  const [showPublishConfirm, setShowPublishConfirm] = useState(false);
  const [showTestSend, setShowTestSend] = useState(false);
  const [activeLocale, setActiveLocale] = useState("default");
  const previewTimeoutRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const resizeRef = useRef<{ startX: number; startWidth: number } | null>(
    null
  );

  const [builderDocument, setBuilderDocument] = useState<BuilderDocument | null>(
    null
  );
  const [codeOverride, setCodeOverride] = useState("");
  const [selectedBlockId, setSelectedBlockId] = useState<string | null>(null);
  const [previewSplitMode, setPreviewSplitMode] =
    useState<PreviewSplitMode>("ratio");
  const [previewPanelWidthPx, setPreviewPanelWidthPx] = useState<number>(
    MIN_PANEL_WIDTH
  );
  const [isResizeDragging, setIsResizeDragging] = useState(false);
  const [draggedBlockId, setDraggedBlockId] = useState<string | null>(null);
  const [blockDropIndex, setBlockDropIndex] = useState<number | null>(null);
  const draggedBlockIdRef = useRef<string | null>(null);
  const layoutSplitRef = useRef<HTMLDivElement | null>(null);
  const previewPanelWrapRef = useRef<HTMLDivElement | null>(null);
  const blockEditorRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const previewStageRef = useRef<HTMLDivElement | null>(null);
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

  const eventVariableTokens = useMemo<TemplateVariable[]>(() => {
    const raw = templateTypeQuery.data?.variable_schema;
    const extracted = collectEventVariablesFromSchema(raw as unknown);

    return Array.from(new Set(extracted)).map((name) => ({
        id: `event-${name}`,
        token: makeVariableToken(name, "event"),
        label: name,
        hint: "Variable de evento",
        category: "event",
      }));
  }, [templateTypeQuery.data]);

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
                hint: field.description || "Variable de injector",
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
  }, [api, scopedPath, injectorItems]);

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

  useEffect(() => {
    const node = previewStageRef.current;
    if (!node || typeof ResizeObserver === "undefined") {
      return;
    }

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
    return () => observer.disconnect();
  }, []);

  useLayoutEffect(() => {
    if (!builderDocument || editorMode !== "visual") {
      return;
    }

    for (const block of builderDocument.blocks) {
      if (block.type !== "text" && block.type !== "button") {
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
  }

  function handleCodeChange(value: string) {
    setCodeOverride(value);
    triggerPreview(value);
  }

  function handleSaveDraft() {
    const formData = getValues();

    const bodyPayload =
      editorMode === "visual"
        ? (builderDocument ? buildTemplateMjml(builderDocument) : codeMjml)
        : codeMjml;

    const body: CreateTemplateVersionRequest = {
      subject: formData.subject,
      preview_text: formData.preview_text || undefined,
      from_name: formData.from_name,
      reply_to: formData.reply_to || undefined,
      body_mjml: bodyPayload,
      default_locale: version?.default_locale ?? "en",
    };

    if (editorMode === "visual" && builderDocument) {
      body.editor_data = builderDocument;
    }

    saveMutation
      .mutateAsync(body)
      .then(() => {
        toast.success("Draft saved");
      })
      .catch(() => {
        toast.error("Failed to save draft");
      });
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

  function addBlock(type: BuilderBlockType) {
    if (!builderDocument || !isDraft) return;

    const id = nowId();
    let newBlock: BuilderBlock;

    if (type === "text") {
      newBlock = {
        id,
        type: "text",
        segments: [createTextSegment("")],
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
            segments: [createTextSegment("")],
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

  function getTextLikeBlockSegments(blockId: string) {
    if (!builderDocument) return;
    const block = builderDocument.blocks.find((candidate) => candidate.id === blockId);
    if (!block || (block.type !== "text" && block.type !== "button")) {
      return null;
    }
    return block.segments;
  }

  function updateTextLikeBlockSegments(
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
    const current = getTextLikeBlockSegments(blockId);
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
        if (block.id !== blockId || (block.type !== "text" && block.type !== "button")) {
          return block;
        }
        return {
          ...block,
          segments: normalized,
        };
      }),
    });
  }

  function handleBlockEditorInput(blockId: string, editor: HTMLDivElement) {
    const parsed = parseSegmentsFromEditorNode(editor);
    updateTextLikeBlockSegments(blockId, parsed);
  }

  function insertVariableIntoEditor(
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
    updateTextLikeBlockSegments(blockId, nextSegments, selection.start + 1);
    return true;
  }

  function appendTemplateVariableToBlock(
    blockId: string,
    variable: TemplateVariable
  ) {
    if (!builderDocument || !isDraft) return;

    const targetId =
      builderDocument.blocks.find((block) => block.id === blockId)?.id ??
      builderDocument.blocks.find(
        (block) => block.type === "text" || block.type === "button"
      )?.id ??
      builderDocument.blocks[0]?.id;

    if (!targetId) return;

    setSelectedBlockId(targetId);

    const editor = blockEditorRefs.current[targetId];
    if (editor && insertVariableIntoEditor(targetId, editor, variable)) {
      return;
    }

    const token = normalizeVariableToken(variable.token);
    if (!token) return;
    const tokenSegment = createTokenSegment(token, variable.category, variable.label);
    const currentSegments = getTextLikeBlockSegments(targetId);
    if (!currentSegments) return;
    const end = countSegmentsUnits(currentSegments);
    const nextSegments = replaceSegmentsUnitRange(currentSegments, end, end, [
      tokenSegment,
    ]);
    updateTextLikeBlockSegments(targetId, nextSegments, end + 1);
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
    updateTextLikeBlockSegments(blockId, nextSegments, deleteStart);
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
      updateTextLikeBlockSegments(blockId, nextSegments, selectionRange.start);
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
    updateTextLikeBlockSegments(blockId, nextSegments, nextCaret);
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
    insertVariableIntoEditor(blockId, editor, variable);
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
    <div className="flex flex-col h-full animate-in fade-in duration-300">
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
            <div className="flex items-center rounded-md border h-8 overflow-hidden">
              <button
                className={`px-2 h-full font-mono text-[11px] font-medium ${
                  activeLocale === "default"
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => setActiveLocale("default")}
              >
                {version.default_locale}
              </button>
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
              <Button
                variant="outline"
                size="sm"
                onClick={handleSaveDraft}
                disabled={saveMutation.isPending}
              >
                <Save className="h-4 w-4 mr-1.5" />
                Save Draft
              </Button>
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
        <div className="flex flex-col flex-1 border-r min-w-0">
          {editorMode === "visual" ? (
            <div className="flex h-full">
              <div className={`shrink-0 ${DEFAULT_BLOCK_WIDTH} border-r p-3 overflow-auto bg-muted/20`}>
                {/* === Variables (event) === */}
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-semibold text-muted-foreground">Variables</h4>
                  <MousePointer className="h-3.5 w-3.5 text-muted-foreground" />
                </div>
                <div className="mt-3 space-y-2">
                  {templateVariables.length > 0 ? (
                    templateVariables
                      .filter((item) => item.category === "event")
                      .map((item) => (
                        <button
                          type="button"
                          key={item.id}
                          className="w-full rounded-md border bg-background p-2 text-xs text-left disabled:opacity-60 disabled:cursor-not-allowed"
                          draggable
                          onDragStart={(event) => onVariableCardDragStart(event, item)}
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

                {/* === Blocks (collapsible) === */}
                <button
                  type="button"
                  className="flex items-center gap-1 mt-4 mb-2 w-full text-left"
                  onClick={() => setBlocksOpen((prev) => !prev)}
                >
                  {blocksOpen ? (
                    <ChevronDown className="h-3 w-3 text-muted-foreground" />
                  ) : (
                    <ChevronRight className="h-3 w-3 text-muted-foreground" />
                  )}
                  <h4 className="text-xs font-semibold text-muted-foreground">Bloques</h4>
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
                      <Type className="h-3.5 w-3.5 mr-1" /> Texto
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!isDraft}
                      onClick={() => addBlock("button")}
                      className="h-8"
                    >
                      Botón
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!isDraft}
                      onClick={() => addBlock("image")}
                      className="h-8"
                    >
                      <ImageIcon className="h-3.5 w-3.5 mr-1" /> Imagen
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!isDraft}
                      onClick={() => addBlock("divider")}
                      className="h-8"
                    >
                      <Minus className="h-3.5 w-3.5 mr-1" /> Divider
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!isDraft}
                      onClick={() => addBlock("spacer")}
                      className="h-8 col-span-2"
                    >
                      <Grip className="h-3.5 w-3.5 mr-1" /> Espaciado
                    </Button>
                  </div>
                )}

                {/* === Injectors (grouped, collapsible, searchable) === */}
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
                      <h4 className="text-xs font-semibold text-muted-foreground mt-4 mb-2">Injectors</h4>
                      <div className="relative mb-2">
                        <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3 w-3 text-muted-foreground" />
                        <Input
                          value={injectorSearch}
                          onChange={(e) => setInjectorSearch(e.target.value)}
                          placeholder="Buscar..."
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
                                              draggable
                                              onDragStart={(event) => onVariableCardDragStart(event, item)}
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
                            <p className="text-xs text-muted-foreground py-1">Sin resultados</p>
                          )}
                        </div>
                      </TooltipProvider>
                    </>
                  );
                })()}
              </div>

              <div className="flex-1 overflow-auto p-4">
                {builderDocument ? (
                  <div className="space-y-2">
                    {builderDocument.blocks.map((block, index) => {
                      const isActive = selectedBlockId === block.id;
                      const canReceiveVariable =
                        block.type === "text" || block.type === "button";

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
                            <div className="flex items-start justify-between mb-2">
                              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                                <button
                                  type="button"
                                  className="inline-flex h-5 w-5 items-center justify-center rounded hover:bg-muted cursor-grab active:cursor-grabbing"
                                  draggable={isDraft}
                                  onDragStart={(event) =>
                                    onBlockHandleDragStart(event, block.id)
                                  }
                                  onDragEnd={handleBlockDragEnd}
                                  onClick={(event) => event.stopPropagation()}
                                  title="Reordenar bloque"
                                >
                                  <GripVertical className="h-3.5 w-3.5" />
                                </button>
                                {block.type}
                              </div>
                              {isDraft ? (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  className="h-7 px-2"
                                  onClick={() => removeBlock(block.id)}
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              ) : null}
                            </div>

                            {(block.type === "text" || block.type === "button") && (
                              <>
                                <Label className="text-xs">Contenido</Label>
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
                                    data-placeholder="Texto"
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
                                <Label className="text-xs">Altura</Label>
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
                                Sin propiedades
                              </div>
                            ) : null}
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
            </div>
          ) : (
            <div className="flex flex-1 min-h-0">
              <div className="flex-1 border-b">
                <MonacoEditorWrapper
                  value={codeMjml}
                  onChange={handleCodeChange}
                  readOnly={!isDraft}
                />
              </div>
            </div>
          )}

          <div className="border-t bg-card p-4 shrink-0">
            <h4 className="text-sm font-semibold mb-3">Metadata</h4>
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
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs font-medium">Reply-To</Label>
                <Input
                  {...register("reply_to")}
                  className="h-8 text-sm"
                  readOnly={!isDraft}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Right panel */}
        <div
          ref={previewPanelWrapRef}
          className="relative flex shrink-0 min-w-0"
          style={{
            width:
              previewSplitMode === "ratio"
                ? `calc(50% + ${PANEL_RESIZER_WIDTH / 2}px)`
                : `${previewPanelWidthPx + PANEL_RESIZER_WIDTH}px`,
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
            <div className="flex-1 bg-slate-100 p-6 overflow-hidden">
              <div
                ref={previewStageRef}
                className="flex h-full w-full items-center justify-center overflow-hidden"
              >
                {previewFrameUrl ? (
                  <div
                    className="relative overflow-hidden rounded-md border border-border/60 bg-white"
                    style={{
                      width: `${previewScaledWidth}px`,
                      height: `${previewScaledHeight}px`,
                    }}
                  >
                    <iframe
                      ref={previewIframeRef}
                      src={previewFrameUrl}
                      onLoad={handlePreviewIframeLoad}
                      className="pointer-events-none border-0 bg-white"
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
                    className="flex items-center justify-center text-sm text-muted-foreground rounded-md border bg-white"
                    style={{
                      width: `${previewScaledWidth}px`,
                      height: `${previewScaledHeight}px`,
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
      theme="vs-dark"
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
        <div className="w-[480px] bg-slate-100 border-l" />
      </div>
    </div>
  );
}
