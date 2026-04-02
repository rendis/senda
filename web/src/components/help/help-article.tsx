import { notFound } from "next/navigation";
import Link from "next/link";
import { DocsBody } from "fumadocs-ui/page";
import { PageShell } from "@/components/shared/page-shell";
import { Card, CardContent } from "@/components/ui/card";
import { HelpHomeCards } from "@/components/help/help-home-cards";
import { getHelpMDXComponents } from "@/components/help/mdx-components";
import { getHelpArticle, getWorkspaceApiDocsHref } from "@/lib/help";

interface HelpArticleProps {
  slug?: string[];
  scopeLabel: string;
  basePath: string;
  tenantCode?: string;
  workspaceCode?: string;
}

export async function HelpArticle({
  slug,
  scopeLabel,
  basePath,
  tenantCode,
  workspaceCode,
}: HelpArticleProps) {
  const article = getHelpArticle(slug);
  if (!article) notFound();

  const Mdx = (await article.load()).default;

  const breadcrumbs: {
    label: string;
    href?: string;
  }[] = [
    { label: scopeLabel, href: basePath.replace(/\/help$/, "") },
    { label: "Help Center", href: basePath },
  ];

  if (article.key !== "index") {
    breadcrumbs.push({ label: article.title });
  }

  const apiDocsHref =
    tenantCode && workspaceCode
      ? getWorkspaceApiDocsHref(tenantCode, workspaceCode)
      : null;

  return (
    <PageShell
      title={article.title}
      description={article.description}
      breadcrumbs={breadcrumbs}
      actions={
        apiDocsHref ? (
          <LinkButton href={apiDocsHref} label="Open API Docs" />
        ) : undefined
      }
    >
      <div className="space-y-6">
        {article.key === "index" && <HelpHomeCards basePath={basePath} />}
        <Card>
          <CardContent className="pt-6">
            <DocsBody>
              <Mdx components={getHelpMDXComponents()} />
            </DocsBody>
          </CardContent>
        </Card>
      </div>
    </PageShell>
  );
}

function LinkButton({ href, label }: { href: string; label: string }) {
  return (
    <Link
      href={href}
      className="inline-flex items-center rounded-md border px-3 py-2 text-sm font-medium transition-colors hover:bg-accent"
    >
      {label}
    </Link>
  );
}
