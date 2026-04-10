import { guessSegmentCategory, normalizeVariableToken } from "./template-token-segments.ts";

function findMatchingSpanCloseIndex(html: string, searchFrom: number) {
  const spanTagPattern = /<\/?span\b[^>]*>/gi;
  spanTagPattern.lastIndex = searchFrom;
  let depth = 1;

  for (;;) {
    const tagMatch = spanTagPattern.exec(html);
    if (!tagMatch) {
      return -1;
    }

    const tag = tagMatch[0].toLowerCase();
    if (tag.startsWith("</span")) {
      depth -= 1;
      if (depth === 0) {
        return tagMatch.index + tagMatch[0].length;
      }
      continue;
    }

    depth += 1;
  }
}

export function mjmlVarsToTiptapHtml(html: string): string {
  return html.replace(/\{\{([^}]+)\}\}/g, (_match, rawToken: string) => {
    const token = normalizeVariableToken(rawToken.trim());
    const category = guessSegmentCategory(token);
    const label = token;
    return `<span data-variable-token="${token}" data-category="${category}">${label}</span>`;
  });
}

export function tiptapHtmlToMjmlVars(html: string): string {
  if (!html || !html.includes("data-variable-token")) {
    return html;
  }

  const tokenSpanPattern = /<span\b[^>]*data-variable-token="([^"]*)"[^>]*>/gi;
  let result = "";
  let cursor = 0;

  for (;;) {
    tokenSpanPattern.lastIndex = cursor;
    const match = tokenSpanPattern.exec(html);
    if (!match) {
      result += html.slice(cursor);
      break;
    }

    const [openTag, token] = match;
    const matchStart = match.index;
    const openTagEnd = matchStart + openTag.length;
    const closeTagEnd = findMatchingSpanCloseIndex(html, openTagEnd);

    if (closeTagEnd === -1) {
      result += html.slice(cursor);
      break;
    }

    result += html.slice(cursor, matchStart);
    result += `{{${token}}}`;
    cursor = closeTagEnd;
  }

  return result;
}
