import type { ComponentType } from "react";
import type { MDXComponents } from "mdx/types";

type MDXModule = {
  default: ComponentType<{ components?: MDXComponents }>;
};

export type HelpArticleKey =
  | "index"
  | "members"
  | "api-keys"
  | "ses-tracking"
  | "send-flow"
  | "faq";

export interface HelpArticleDefinition {
  key: HelpArticleKey;
  title: string;
  description: string;
  load: () => Promise<MDXModule>;
}

const articleDefinitions: Record<HelpArticleKey, HelpArticleDefinition> = {
  index: {
    key: "index",
    title: "Help Center",
    description: "Guides, API references, and troubleshooting for Senda.",
    load: () => import("../../content/help/index.mdx"),
  },
  members: {
    key: "members",
    title: "Members",
    description: "How scope-aware member access and role visibility work.",
    load: () => import("../../content/help/members.mdx"),
  },
  "api-keys": {
    key: "api-keys",
    title: "API Keys",
    description: "How to generate, reveal, and use workspace-scoped API keys and their data-plane endpoints.",
    load: () => import("../../content/help/api-keys.mdx"),
  },
  "ses-tracking": {
    key: "ses-tracking",
    title: "SES tracking",
    description: "What Senda provisions for SES tracking and what is reused.",
    load: () => import("../../content/help/ses-tracking.mdx"),
  },
  "send-flow": {
    key: "send-flow",
    title: "Send flow",
    description: "How templates, injectors, and adapters resolve during send.",
    load: () => import("../../content/help/send-flow.mdx"),
  },
  faq: {
    key: "faq",
    title: "FAQ",
    description: "Quick answers for common product and integration questions.",
    load: () => import("../../content/help/faq.mdx"),
  },
};

export const helpArticles = Object.values(articleDefinitions);

export function getScopedBasePath(pathname: string): string {
  const workspaceMatch = pathname.match(/^\/t\/[^/]+\/w\/[^/]+/);
  if (workspaceMatch) return workspaceMatch[0];

  const tenantMatch = pathname.match(/^\/t\/[^/]+/);
  if (tenantMatch) return tenantMatch[0];

  if (pathname.startsWith("/global")) return "/global";

  return "/global";
}

export function getHelpBasePath(pathname: string): string {
  return `${getScopedBasePath(pathname)}/help`;
}

export function getContextualHelpHref(pathname: string): string {
  const helpBase = getHelpBasePath(pathname);
  const stripped = pathname.replace(getScopedBasePath(pathname), "");

  if (stripped.startsWith("/members")) return `${helpBase}/members`;
  if (stripped.startsWith("/api-keys")) return `${helpBase}/api-keys`;
  if (stripped.startsWith("/adapters")) return `${helpBase}/ses-tracking`;
  if (stripped.startsWith("/templates") || stripped.startsWith("/injectors")) {
    return `${helpBase}/send-flow`;
  }

  return helpBase;
}

export function getHelpArticle(slug?: string[]): HelpArticleDefinition | null {
  if (!slug || slug.length === 0) return articleDefinitions.index;

  const joined = slug.join("/") as HelpArticleKey;
  return articleDefinitions[joined] ?? null;
}

export function getWorkspaceApiDocsHref(
  tenantCode: string,
  workspaceCode: string,
): string {
  return `/t/${tenantCode}/w/${workspaceCode}/api-docs`;
}
