import { Node, mergeAttributes } from "@tiptap/core";

export interface VariableTokenOptions {
  HTMLAttributes: Record<string, unknown>;
}

type VariableTokenElement = {
  getAttribute: (name: string) => string | null;
  textContent?: string | null;
};

export function readVariableTokenAttrs(element: VariableTokenElement) {
  const token = element.getAttribute("data-variable-token")?.trim() ?? "";
  const label =
    element.getAttribute("data-label")?.trim() ||
    element.textContent?.trim() ||
    token;
  const category =
    element.getAttribute("data-category") === "injector" ? "injector" : "event";

  return {
    token,
    label,
    category,
  };
}

export function getVariableTokenClassName(category: "event" | "injector") {
  return [
    "inline-flex max-w-full min-w-0 items-center overflow-hidden text-ellipsis whitespace-nowrap rounded border border-dashed px-1.5 py-0.5 text-xs align-middle select-none",
    category === "injector"
      ? "border-violet-400 bg-violet-50 text-violet-700 dark:border-violet-600 dark:bg-violet-950 dark:text-violet-300"
      : "border-input bg-muted text-foreground",
  ].join(" ");
}

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    variableToken: {
      insertVariable: (attrs: {
        token: string;
        label: string;
        category: "event" | "injector";
      }) => ReturnType;
    };
  }
}

export const VariableToken = Node.create<VariableTokenOptions>({
  name: "variableToken",
  group: "inline",
  inline: true,
  atom: true,
  draggable: true,
  selectable: true,

  addOptions() {
    return { HTMLAttributes: {} };
  },

  addAttributes() {
    return {
      token: { default: "" },
      label: { default: "" },
      category: { default: "event" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-variable-token]",
        getAttrs: (element) => readVariableTokenAttrs(element as VariableTokenElement),
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    const cat = (node.attrs.category === "injector" ? "injector" : "event") as "event" | "injector";
    const label = (node.attrs.label as string) || (node.attrs.token as string);
    return [
      "span",
      mergeAttributes(this.options.HTMLAttributes, HTMLAttributes, {
        "data-variable-token": node.attrs.token,
        "data-label": label,
        "data-category": cat,
        class: getVariableTokenClassName(cat),
        contenteditable: "false",
        title: label,
        "aria-label": label,
      }),
      label,
    ];
  },

  addCommands() {
    return {
      insertVariable:
        (attrs) =>
        ({ chain }) =>
          chain()
            .insertContent({
              type: this.name,
              attrs,
            })
            .run(),
    };
  },
});
