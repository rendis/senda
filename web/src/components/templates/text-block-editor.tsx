"use client";

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { TextStyle, FontSize, Color, FontFamily } from "@tiptap/extension-text-style";
import TextAlign from "@tiptap/extension-text-align";
import { VariableToken } from "./tiptap-variable-token";
import {
  Bold,
  Italic,
  Underline as UnderlineIcon,
  Strikethrough,
  AlignLeft,
  AlignCenter,
  AlignRight,
  AlignJustify,
  Palette,
  X,
} from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

const VARIABLE_DND_MIME = "application/x-senda-variable";

const DEFAULT_FONT_FAMILY = "Arial, sans-serif";
const DEFAULT_FONT_SIZE = "14";
const DEFAULT_COLOR = "#000000";

const FONT_SIZES = ["12", "14", "16", "18", "20", "24", "28", "32", "36", "48"];

const EMAIL_SAFE_FONTS = [
  { label: "Arial", value: "Arial, sans-serif" },
  { label: "Verdana", value: "Verdana, Geneva, sans-serif" },
  { label: "Trebuchet MS", value: "'Trebuchet MS', Helvetica, sans-serif" },
  { label: "Georgia", value: "Georgia, serif" },
  { label: "Times New Roman", value: "'Times New Roman', Times, serif" },
  { label: "Courier New", value: "'Courier New', Courier, monospace" },
];

const COLOR_PALETTE = [
  "#000000",
  "#434343",
  "#666666",
  "#999999",
  "#e74c3c",
  "#e67e22",
  "#f1c40f",
  "#2ecc71",
  "#1abc9c",
  "#3498db",
  "#9b59b6",
  "#ffffff",
];

type TextBlockAlign = "left" | "center" | "right" | "justify";

export interface TextBlockEditorHandle {
  insertVariable: (attrs: { token: string; label: string; category: "event" | "injector" }) => void;
}

interface TextBlockEditorProps {
  content: string;
  onChange: (html: string, align?: TextBlockAlign) => void;
  align: TextBlockAlign;
  disabled?: boolean;
  placeholder?: string;
  onFocus?: () => void;
}

export const TextBlockEditor = forwardRef<TextBlockEditorHandle, TextBlockEditorProps>(
  function TextBlockEditor(
    {
      content,
      onChange,
      align,
      disabled = false,
      placeholder,
      onFocus,
    },
    ref,
  ) {
  const t = useTranslations("editor");
  const effectivePlaceholder = placeholder ?? t("textPlaceholder");
  const [colorOpen, setColorOpen] = useState(false);
  const [customColor, setCustomColor] = useState("");
  const isInternalUpdate = useRef(false);
  const [, setTick] = useState(0);

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: false,
        bulletList: false,
        orderedList: false,
        blockquote: false,
        codeBlock: false,
        code: false,
        horizontalRule: false,
        listItem: false,
      }),
      TextStyle,
      FontSize,
      Color,
      FontFamily,
      TextAlign.configure({
        types: ["paragraph"],
        alignments: ["left", "center", "right", "justify"],
      }),
      VariableToken,
    ],
    immediatelyRender: false,
    content,
    editable: !disabled,
    onCreate: ({ editor }) => {
      if (editor.isEmpty) {
        editor.chain()
          .setFontFamily(DEFAULT_FONT_FAMILY)
          .setFontSize(`${DEFAULT_FONT_SIZE}px`)
          .run();
      }
    },
    onUpdate: ({ editor }) => {
      isInternalUpdate.current = true;
      onChange(editor.getHTML());
    },
    onTransaction() {
      setTick((t) => t + 1);
    },
    onFocus: () => onFocus?.(),
    editorProps: {
      attributes: {
        "data-placeholder": effectivePlaceholder,
      },
      handleDrop: (view, event) => {
        const raw = event.dataTransfer?.getData(VARIABLE_DND_MIME);
        if (!raw) return false;
        try {
          const data = JSON.parse(raw);
          const coords = { left: event.clientX, top: event.clientY };
          const pos = view.posAtCoords(coords);
          if (!pos) return false;
          const { tr } = view.state;
          const node = view.state.schema.nodes.variableToken.create({
            token: data.token,
            label: data.label,
            category: data.category,
          });
          tr.insert(pos.pos, node);
          view.dispatch(tr);
          return true;
        } catch {
          return false;
        }
      },
      handleDOMEvents: {
        dragover: (_view, event) => {
          if (event.dataTransfer?.types.includes(VARIABLE_DND_MIME)) {
            event.preventDefault();
            event.dataTransfer.dropEffect = "copy";
            return true;
          }
          return false;
        },
      },
    },
  });

  useImperativeHandle(ref, () => ({
    insertVariable(attrs) {
      if (!editor) return;
      editor.chain().focus().insertVariable(attrs).run();
    },
  }), [editor]);

  // Sync content from parent (e.g., when switching blocks or undo)
  useEffect(() => {
    if (!editor) return;
    if (isInternalUpdate.current) {
      isInternalUpdate.current = false;
      return;
    }
    const current = editor.getHTML();
    if (current !== content) {
      editor.commands.setContent(content, { emitUpdate: false });
    }
  }, [editor, content]);

  // Sync editable state
  useEffect(() => {
    if (editor) {
      editor.setEditable(!disabled);
    }
  }, [editor, disabled]);

  // Keep editor alignment synchronized with external block state.
  useEffect(() => {
    if (!editor) return;

    const currentAlign: TextBlockAlign =
      editor.isActive({ textAlign: "center" }) ? "center"
      : editor.isActive({ textAlign: "right" }) ? "right"
      : editor.isActive({ textAlign: "justify" }) ? "justify"
      : "left";

    if (currentAlign === align) {
      return;
    }

    editor.chain().setTextAlign(align).run();
  }, [editor, align]);

  const setAlign = useCallback(
    (a: TextBlockAlign) => {
      if (!editor) return;
      editor.chain().focus().setTextAlign(a).run();
      // Emit both html + align in a single call to avoid stale state
      isInternalUpdate.current = true;
      onChange(editor.getHTML(), a);
    },
    [editor, onChange],
  );

  const applyColor = useCallback(
    (color: string) => {
      if (!editor) return;
      editor.chain().focus().setColor(color).run();
      setColorOpen(false);
    },
    [editor],
  );

  if (!editor) return null;

  const ts = editor.getAttributes("textStyle");
  const currentFontFamily = ts?.fontFamily || DEFAULT_FONT_FAMILY;
  const currentFontSize = ts?.fontSize ? String(parseInt(ts.fontSize)) : DEFAULT_FONT_SIZE;
  const currentColor = ts?.color || DEFAULT_COLOR;
  const currentAlign: TextBlockAlign =
    editor.isActive({ textAlign: "center" }) ? "center"
    : editor.isActive({ textAlign: "right" }) ? "right"
    : editor.isActive({ textAlign: "justify" }) ? "justify"
    : "left";

  return (
    <div className="space-y-1">
      {/* Toolbar */}
      {!disabled && (
        <TooltipProvider delayDuration={300}>
          <div className="flex flex-wrap items-center gap-0.5 rounded-md border border-input bg-muted/40 px-1 py-0.5">
            {/* Font family */}
            <Select
              value={currentFontFamily}
              onValueChange={(val) => {
                if (val === "__none__") {
                  editor.chain().focus().unsetFontFamily().run();
                } else {
                  editor.chain().focus().setFontFamily(val).run();
                }
              }}
            >
              <SelectTrigger className="h-7 w-28 border-0 bg-transparent px-1.5 text-xs shadow-none">
                <SelectValue placeholder="Font" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">
                  <span className="text-muted-foreground">Default</span>
                </SelectItem>
                {EMAIL_SAFE_FONTS.map((f) => (
                  <SelectItem key={f.value} value={f.value}>
                    <span style={{ fontFamily: f.value }}>{f.label}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {/* Font size */}
            <Select
              value={currentFontSize}
              onValueChange={(val) => {
                if (val === "__none__") {
                  editor.chain().focus().unsetFontSize().run();
                } else {
                  editor.chain().focus().setFontSize(`${val}px`).run();
                }
              }}
            >
              <SelectTrigger className="h-7 w-16 border-0 bg-transparent px-1.5 text-xs shadow-none">
                <SelectValue placeholder="Size" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">
                  <span className="text-muted-foreground">Default</span>
                </SelectItem>
                {FONT_SIZES.map((s) => (
                  <SelectItem key={s} value={s}>
                    {s}px
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Separator orientation="vertical" className="mx-0.5 h-5" />

            {/* Bold */}
            <ToolbarButton
              icon={Bold}
              label="Bold"
              shortcut="Cmd+B"
              active={editor.isActive("bold")}
              onPress={() => editor.chain().focus().toggleBold().run()}
            />

            {/* Italic */}
            <ToolbarButton
              icon={Italic}
              label="Italic"
              shortcut="Cmd+I"
              active={editor.isActive("italic")}
              onPress={() => editor.chain().focus().toggleItalic().run()}
            />

            {/* Underline */}
            <ToolbarButton
              icon={UnderlineIcon}
              label="Underline"
              shortcut="Cmd+U"
              active={editor.isActive("underline")}
              onPress={() => editor.chain().focus().toggleUnderline().run()}
            />

            {/* Strikethrough */}
            <ToolbarButton
              icon={Strikethrough}
              label="Strikethrough"
              active={editor.isActive("strike")}
              onPress={() => editor.chain().focus().toggleStrike().run()}
            />

            <Separator orientation="vertical" className="mx-0.5 h-5" />

            {/* Color picker */}
            <Popover open={colorOpen} onOpenChange={setColorOpen}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className={cn(
                        "inline-flex h-7 w-7 items-center justify-center rounded-sm text-sm hover:bg-accent",
                      )}
                    >
                      <div className="flex flex-col items-center gap-0">
                        <Palette className="h-3.5 w-3.5" />
                        <div
                          className="h-0.5 w-3.5 rounded-full"
                          style={{ backgroundColor: currentColor }}
                        />
                      </div>
                    </button>
                  </PopoverTrigger>
                </TooltipTrigger>
                <TooltipContent>Color</TooltipContent>
              </Tooltip>
              <PopoverContent className="w-auto p-3" align="start">
                <div className="grid grid-cols-6 gap-1.5">
                  {COLOR_PALETTE.map((c) => (
                    <button
                      key={c}
                      type="button"
                      className={cn(
                        "h-6 w-6 rounded-sm border border-input transition-transform hover:scale-110",
                        currentColor === c && "ring-2 ring-ring ring-offset-1",
                      )}
                      style={{ backgroundColor: c }}
                      onClick={() => applyColor(c)}
                    />
                  ))}
                </div>
                <div className="mt-2 flex items-center gap-1.5">
                  <Input
                    className="h-7 flex-1 text-xs"
                    placeholder="#hex"
                    value={customColor}
                    onChange={(e) => setCustomColor(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && customColor) {
                        applyColor(customColor);
                        setCustomColor("");
                      }
                    }}
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 px-2"
                    onClick={() => {
                      if (customColor) {
                        applyColor(customColor);
                        setCustomColor("");
                      }
                    }}
                  >
                    Ok
                  </Button>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-7 w-7 px-0"
                        onClick={() => {
                          editor.chain().focus().unsetColor().run();
                          setColorOpen(false);
                        }}
                      >
                        <X className="h-3.5 w-3.5" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>Reset color</TooltipContent>
                  </Tooltip>
                </div>
              </PopoverContent>
            </Popover>

            <Separator orientation="vertical" className="mx-0.5 h-5" />

            {/* Alignment */}
            <ToolbarButton
              icon={AlignLeft}
              label="Align left"
              active={currentAlign === "left"}
              onPress={() => setAlign("left")}
            />
            <ToolbarButton
              icon={AlignCenter}
              label="Align center"
              active={currentAlign === "center"}
              onPress={() => setAlign("center")}
            />
            <ToolbarButton
              icon={AlignRight}
              label="Align right"
              active={currentAlign === "right"}
              onPress={() => setAlign("right")}
            />
            <ToolbarButton
              icon={AlignJustify}
              label="Justify"
              active={currentAlign === "justify"}
              onPress={() => setAlign("justify")}
            />
          </div>
        </TooltipProvider>
      )}

      {/* Editor */}
      <div
        className={cn(
          "rounded-md border border-input bg-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2",
          disabled && "opacity-60",
        )}
      >
        <EditorContent
          editor={editor}
          className="[&_.tiptap]:min-h-6 [&_.tiptap]:px-2 [&_.tiptap]:py-1.5 [&_.tiptap]:leading-normal [&_.tiptap]:min-w-0 [&_.tiptap]:w-full [&_.tiptap]:outline-none [&_.tiptap]:text-sm [&_.tiptap]:whitespace-pre-wrap [&_.tiptap]:break-words [&_.tiptap]:[overflow-wrap:anywhere] [&_.tiptap_p]:my-0 [&_.tiptap_p]:min-h-[1lh] [&_.tiptap_p]:leading-normal [&_.tiptap_p]:min-w-0 [&_.tiptap_p]:max-w-full [&_.tiptap_span[data-variable-token]]:max-w-full [&_.tiptap_.ProseMirror-trailingBreak]:block"
        />
      </div>
    </div>
  );
});

function ToolbarButton({
  icon: Icon,
  label,
  shortcut,
  active,
  onPress,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  shortcut?: string;
  active?: boolean;
  onPress: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className={cn(
            "inline-flex h-7 w-7 items-center justify-center rounded-sm text-sm hover:bg-accent",
            active && "bg-primary/15 text-primary",
          )}
          onMouseDown={(e) => {
            e.preventDefault();
            onPress();
          }}
        >
          <Icon className="h-3.5 w-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent>
        {label}
        {shortcut ? ` (${shortcut})` : ""}
      </TooltipContent>
    </Tooltip>
  );
}
