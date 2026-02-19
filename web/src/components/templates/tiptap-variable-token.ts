import { Node, mergeAttributes } from "@tiptap/core";

export interface VariableTokenOptions {
  HTMLAttributes: Record<string, unknown>;
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
    return [{ tag: "span[data-variable-token]" }];
  },

  renderHTML({ node, HTMLAttributes }) {
    const cat = node.attrs.category as string;
    const label = (node.attrs.label as string) || (node.attrs.token as string);
    return [
      "span",
      mergeAttributes(this.options.HTMLAttributes, HTMLAttributes, {
        "data-variable-token": node.attrs.token,
        "data-category": cat,
        class: [
          "inline-flex items-center rounded border border-dashed px-1.5 py-0.5 text-xs align-middle select-none",
          cat === "injector"
            ? "border-violet-400 bg-violet-50 text-violet-700 dark:border-violet-600 dark:bg-violet-950 dark:text-violet-300"
            : "border-input bg-muted text-foreground",
        ].join(" "),
        contenteditable: "false",
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
