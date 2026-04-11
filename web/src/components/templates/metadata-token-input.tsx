"use client";

import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
} from "react";
import { cn } from "@/lib/utils";
import { METADATA_TOKEN_INPUT_EDITOR_CLASSNAME } from "./metadata-token-input-styles";
import type {
  TemplateTokenSegment,
  TokenCategory,
} from "./template-token-segments";
import {
  createTextSegment,
  createTokenSegment,
  ensureUniqueSegmentIds,
  getTokenChipClassName,
  getTokenChipText,
  guessSegmentCategory,
  mergeAdjacentTextSegments,
  normalizeVariableToken,
  parseTextChunkToSegments,
  renderSegmentsToText,
} from "./template-token-segments";

const VARIABLE_DND_MIME = "application/x-senda-variable";
const CLIPBOARD_SEGMENTS_MIME = "application/x-senda-segments";
const MIME_TEXT_PLAIN = "text/plain";
const TOKEN_SEGMENT_KIND = "token";

type InsertVariablePayload = {
  token: string;
  label: string;
  category: TokenCategory;
};

export interface MetadataTokenInputHandle {
  insertVariable: (attrs: InsertVariablePayload) => void;
}

interface MetadataTokenInputProps {
  value: string;
  onChange: (value: string) => void;
  onFocus?: () => void;
  disabled?: boolean;
  placeholder?: string;
  ariaLabel: string;
  ariaInvalid?: boolean;
  describedBy?: string;
  resolveTokenMeta?: (token: string) => {
    label?: string;
    static?: boolean;
    source?: "database" | "code";
  } | undefined;
  className?: string;
}

function isTokenChipNode(node: Node): node is HTMLElement {
  return (
    node.nodeType === Node.ELEMENT_NODE &&
    (node as HTMLElement).dataset.segmentKind === TOKEN_SEGMENT_KIND
  );
}

function countSegmentUnits(segment: TemplateTokenSegment) {
  if (segment.kind === "token") return 1;
  return segment.text.length;
}

function countSegmentsUnits(segments: TemplateTokenSegment[]) {
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

function getPointUnitOffset(editor: HTMLElement, container: Node, offset: number): number {
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
  return start <= finish ? { start, end: finish } : { start: finish, end: start };
}

function resolveCaretDomPoint(editor: HTMLElement, unitOffset: number) {
  const totalUnits = countUnitsInDomNode(editor);
  let remaining = Math.max(0, Math.min(unitOffset, totalUnits));

  function walk(node: Node): { container: Node; offset: number } | null {
    if (node.nodeType === Node.TEXT_NODE) {
      const text = node.textContent ?? "";
      if (remaining <= text.length) {
        return { container: node, offset: remaining };
      }
      remaining -= text.length;
      return null;
    }

    if (isTokenChipNode(node)) {
      const parent = node.parentNode;
      if (!parent) return null;
      const index = Array.prototype.indexOf.call(parent.childNodes, node);
      if (remaining === 0) {
        return { container: parent, offset: index };
      }
      remaining -= 1;
      if (remaining === 0) {
        return { container: parent, offset: index + 1 };
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
  return { container: editor, offset: editor.childNodes.length };
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

function splitSegmentsAtUnitOffset(segments: TemplateTokenSegment[], unitOffset: number) {
  const clamped = Math.max(0, Math.min(unitOffset, countSegmentsUnits(segments)));
  let remaining = clamped;
  const left: TemplateTokenSegment[] = [];
  const right: TemplateTokenSegment[] = [];

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
        left.push({ ...segment, text: leftText });
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
  segments: TemplateTokenSegment[],
  start: number,
  end: number,
  inserted: TemplateTokenSegment[],
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
    ]),
  );
}

function sliceSegmentsByUnitRange(
  segments: TemplateTokenSegment[],
  start: number,
  end: number,
) {
  const total = countSegmentsUnits(segments);
  const from = Math.max(0, Math.min(start, total));
  const to = Math.max(from, Math.min(end, total));
  const firstSplit = splitSegmentsAtUnitOffset(segments, from);
  const secondSplit = splitSegmentsAtUnitOffset(firstSplit.right, to - from);
  return ensureUniqueSegmentIds(mergeAdjacentTextSegments(secondSplit.left));
}

function parseClipboardSegments(raw: string): TemplateTokenSegment[] | null {
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

    const segments: TemplateTokenSegment[] = [];
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
            typeof segment.label === "string" ? segment.label : segment.token,
          ),
        );
      }
    }
    return segments.length ? mergeAdjacentTextSegments(segments) : null;
  } catch {
    return null;
  }
}

function parseSegmentsFromEditorNode(root: HTMLElement) {
  const segments: TemplateTokenSegment[] = [];

  function collect(nodes: NodeListOf<ChildNode> | ChildNode[]) {
    for (const node of nodes) {
      if (node.nodeType === Node.TEXT_NODE) {
        segments.push(
          ...parseTextChunkToSegments((node.textContent ?? "").replace(/\s*\n+\s*/g, " ")),
        );
        continue;
      }

      if (node.nodeType !== Node.ELEMENT_NODE) {
        continue;
      }

      const element = node as HTMLElement;
      if (element.dataset.segmentKind === TOKEN_SEGMENT_KIND) {
        const token = normalizeVariableToken(
          element.dataset.token ?? element.textContent ?? "",
        );
        if (!token) {
          continue;
        }
        segments.push({
          kind: "token",
          id: element.dataset.segmentId?.trim() || crypto.randomUUID(),
          token,
          label: element.dataset.label?.trim() || token,
          category: element.dataset.category === "injector" ? "injector" : "event",
        });
        continue;
      }

      collect(element.childNodes);
    }
  }

  collect(root.childNodes);
  return ensureUniqueSegmentIds(mergeAdjacentTextSegments(segments));
}

function segmentsMeaningfullyEqual(a: TemplateTokenSegment[], b: TemplateTokenSegment[]) {
  if (a.length !== b.length) return false;
  for (let index = 0; index < a.length; index += 1) {
    const left = a[index];
    const right = b[index];
    if (!left || !right || left.kind !== right.kind) return false;
    if (left.kind === "text" && right.kind === "text" && left.text !== right.text) {
      return false;
    }
    if (
      left.kind === "token" &&
      right.kind === "token" &&
      (left.token !== right.token ||
        left.label !== right.label ||
        left.category !== right.category)
    ) {
      return false;
    }
  }
  return true;
}

function renderSegmentsToEditorNode(
  editor: HTMLElement,
  segments: TemplateTokenSegment[],
  resolveTokenMeta?: MetadataTokenInputProps["resolveTokenMeta"],
) {
  const doc = editor.ownerDocument;
  const fragment = doc.createDocumentFragment();

  for (const segment of segments) {
    if (segment.kind === "text") {
      if (!segment.text) continue;
      fragment.appendChild(doc.createTextNode(segment.text));
      continue;
    }

    const chip = doc.createElement("span");
    const tokenMeta = resolveTokenMeta?.(segment.token);
    const displayLabel = tokenMeta?.label?.trim()
      ? tokenMeta.label
      : getTokenChipText(segment);
    const chipClassName =
      tokenMeta?.source === "code"
        ? "inline-flex max-w-full items-center rounded border border-dashed border-fuchsia-500 bg-fuchsia-50 px-1.5 py-0.5 text-xs align-middle select-none truncate text-fuchsia-700 dark:border-fuchsia-600 dark:bg-fuchsia-950 dark:text-fuchsia-300"
        : getTokenChipClassName(segment.category);
    chip.className = cn(
      chipClassName,
      tokenMeta?.static && "ring-1 ring-amber-500/60 ring-inset",
    );
    chip.contentEditable = "false";
    chip.dataset.segmentKind = TOKEN_SEGMENT_KIND;
    chip.dataset.segmentId = segment.id;
    chip.dataset.token = segment.token;
    chip.dataset.label = displayLabel;
    chip.dataset.category = segment.category;
    chip.textContent = displayLabel;
    chip.title = [
      segment.token,
      tokenMeta?.static ? "static default" : null,
      tokenMeta?.source === "code" ? "code injector" : null,
    ]
      .filter(Boolean)
      .join(" · ");
    fragment.appendChild(chip);
  }

  editor.replaceChildren(fragment);
}

function resolveVariableFromDrop(event: React.DragEvent<HTMLElement>): InsertVariablePayload | null {
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
        return {
          token,
          label:
            typeof parsed.label === "string" && parsed.label.trim()
              ? parsed.label
              : token,
          category: parsed.category === "injector" ? "injector" : "event",
        };
      }
    } catch {
      // no-op
    }
  }

  const token = normalizeVariableToken(event.dataTransfer.getData(MIME_TEXT_PLAIN));
  if (!token || !token.includes(".")) return null;

  return {
    token,
    label: token,
    category: guessSegmentCategory(token),
  };
}

function isVariableDragEvent(dataTransfer: DataTransfer) {
  const types = Array.from(dataTransfer.types);
  return (
    types.includes(VARIABLE_DND_MIME) ||
    types.includes("application/json") ||
    types.includes(MIME_TEXT_PLAIN)
  );
}

export const MetadataTokenInput = forwardRef<
  MetadataTokenInputHandle,
  MetadataTokenInputProps
>(function MetadataTokenInput(
  {
    value,
    onChange,
    onFocus,
    disabled = false,
    placeholder,
    ariaLabel,
    ariaInvalid = false,
    describedBy,
    resolveTokenMeta,
    className,
  },
  ref,
) {
  const editorRef = useRef<HTMLDivElement | null>(null);
  const pendingCaretRestoreRef = useRef<number | null>(null);

  const applySegments = useCallback(
    (segments: TemplateTokenSegment[], nextCaret?: number) => {
      onChange(renderSegmentsToText(segments));
      if (typeof nextCaret === "number") {
        pendingCaretRestoreRef.current = nextCaret;
      }
    },
    [onChange],
  );

  const insertVariable = useCallback(
    (variable: InsertVariablePayload) => {
      const editor = editorRef.current;
      if (!editor || disabled) return;
      editor.focus();
      const tokenSegment = createTokenSegment(variable.token, variable.category, variable.label);
      const liveSegments = parseSegmentsFromEditorNode(editor);
      const selection = getSelectionUnitRange(editor);
      const nextSegments = replaceSegmentsUnitRange(
        liveSegments,
        selection.start,
        selection.end,
        [tokenSegment],
      );
      applySegments(nextSegments, selection.start + 1);
    },
    [applySegments, disabled],
  );

  useImperativeHandle(
    ref,
    () => ({
      insertVariable,
    }),
    [insertVariable],
  );

  useLayoutEffect(() => {
    const editor = editorRef.current;
    if (!editor) {
      pendingCaretRestoreRef.current = null;
      return;
    }
    const nextSegments = parseTextChunkToSegments(value.replace(/\s*\n+\s*/g, " "));
    const normalizedNext =
      nextSegments.length > 0
        ? ensureUniqueSegmentIds(mergeAdjacentTextSegments(nextSegments))
        : [createTextSegment("")];
    const liveSegments = parseSegmentsFromEditorNode(editor);
    if (!segmentsMeaningfullyEqual(liveSegments, normalizedNext)) {
      renderSegmentsToEditorNode(editor, normalizedNext, resolveTokenMeta);
    }
    const pendingCaret = pendingCaretRestoreRef.current;
    if (typeof pendingCaret === "number") {
      setEditorCaretByUnitOffset(editor, pendingCaret);
      pendingCaretRestoreRef.current = null;
    }
  }, [resolveTokenMeta, value]);

  return (
    <div
      className={cn(
        "min-w-0 rounded-md border border-input bg-background px-2 py-1.5 focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2",
        disabled && "cursor-not-allowed opacity-70",
        className,
      )}
    >
      <div
        ref={editorRef}
        role="textbox"
        aria-label={ariaLabel}
        aria-multiline="false"
        aria-invalid={ariaInvalid || undefined}
        aria-describedby={describedBy}
        contentEditable={!disabled}
        suppressContentEditableWarning
        className={METADATA_TOKEN_INPUT_EDITOR_CLASSNAME}
        data-placeholder={placeholder}
        onFocus={() => onFocus?.()}
        onInput={(event) => {
          const segments = parseSegmentsFromEditorNode(event.currentTarget);
          applySegments(segments);
        }}
        onClick={(event) => {
          const target = event.target as HTMLElement | null;
          if (!target || target.dataset.segmentKind !== TOKEN_SEGMENT_KIND) {
            return;
          }
          const selection = window.getSelection();
          if (!selection) return;
          const range = document.createRange();
          range.selectNode(target);
          selection.removeAllRanges();
          selection.addRange(range);
        }}
        onKeyDown={(event) => {
          if (disabled) return;
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
            [],
          );
          applySegments(nextSegments, deleteStart);
        }}
        onCopy={(event) => {
          const editor = event.currentTarget;
          const liveSegments = parseSegmentsFromEditorNode(editor);
          const selectionRange = getSelectionUnitRange(editor);
          if (selectionRange.start === selectionRange.end) {
            return;
          }
          const copiedSegments = sliceSegmentsByUnitRange(
            liveSegments,
            selectionRange.start,
            selectionRange.end,
          );
          const plain = renderSegmentsToText(copiedSegments);

          event.preventDefault();
          event.clipboardData.setData(MIME_TEXT_PLAIN, plain);
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
                    },
              ),
            }),
          );
        }}
        onCut={(event) => {
          if (disabled) return;
          const editor = event.currentTarget;
          const liveSegments = parseSegmentsFromEditorNode(editor);
          const selectionRange = getSelectionUnitRange(editor);
          if (selectionRange.start === selectionRange.end) {
            return;
          }
          const copiedSegments = sliceSegmentsByUnitRange(
            liveSegments,
            selectionRange.start,
            selectionRange.end,
          );
          const plain = renderSegmentsToText(copiedSegments);

          event.preventDefault();
          event.clipboardData.setData(MIME_TEXT_PLAIN, plain);
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
                    },
              ),
            }),
          );
          const nextSegments = replaceSegmentsUnitRange(
            liveSegments,
            selectionRange.start,
            selectionRange.end,
            [],
          );
          applySegments(nextSegments, selectionRange.start);
        }}
        onPaste={(event) => {
          if (disabled) return;
          const editor = event.currentTarget;
          const clipboardSegments = parseClipboardSegments(
            event.clipboardData.getData(CLIPBOARD_SEGMENTS_MIME),
          );
          const fallbackText = event.clipboardData
            .getData(MIME_TEXT_PLAIN)
            .replace(/\s*\n+\s*/g, " ");
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
            insertedSegments,
          );
          const nextCaret = selectionRange.start + countSegmentsUnits(insertedSegments);
          applySegments(nextSegments, nextCaret);
        }}
        onDragOver={(event) => {
          if (disabled || !isVariableDragEvent(event.dataTransfer)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          event.dataTransfer.dropEffect = "copy";
          const editor = event.currentTarget;
          editor.focus();
          placeEditorCaretFromPoint(editor, event.clientX, event.clientY);
        }}
        onDrop={(event) => {
          if (disabled || !isVariableDragEvent(event.dataTransfer)) {
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
          insertVariable(variable);
        }}
      />
    </div>
  );
});
