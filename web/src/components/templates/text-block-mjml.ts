import { tiptapHtmlToMjmlVars } from "./template-variable-html.ts";

type TextBlockAlign = "left" | "center" | "right" | "justify";

const EXPLICIT_TEXT_ALIGN_PATTERN = /text-align\s*:\s*(left|center|right|justify)/i;

export function renderTextBlockToMjml(content: string, align: TextBlockAlign): string {
  const inner = tiptapHtmlToMjmlVars(content).replace(/&quot;/g, "'").trim() || " ";
  const hasExplicitTextAlignment = EXPLICIT_TEXT_ALIGN_PATTERN.test(content);
  const alignAttr =
    !hasExplicitTextAlignment && align !== "left" ? ` align="${align}"` : "";

  return `<mj-text${alignAttr}>${inner}</mj-text>`;
}
