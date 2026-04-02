import { notFound } from "next/navigation";
import Link from "next/link";
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
            <div className="flex-1 space-y-6 text-sm leading-7 text-foreground [&_a]:font-medium [&_a]:text-primary [&_a]:underline-offset-4 hover:[&_a]:underline [&_code]:rounded [&_code]:bg-muted [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em] [&_h1]:text-3xl [&_h1]:font-semibold [&_h1]:tracking-tight [&_h1]:text-foreground [&_h2]:mt-10 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-2xl [&_h2]:font-semibold [&_h2]:tracking-tight [&_h2]:text-foreground [&_h3]:mt-8 [&_h3]:text-xl [&_h3]:font-semibold [&_h3]:text-foreground [&_li]:ml-6 [&_li]:list-disc [&_li]:pl-1 [&_ol]:space-y-2 [&_ol]:pl-6 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:bg-slate-950 [&_pre]:p-4 [&_pre]:text-sm [&_pre]:text-slate-50 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-inherit [&_p]:text-muted-foreground [&_p]:leading-7 [&_ul]:space-y-2 [&_ul]:pl-6">
              <Mdx components={getHelpMDXComponents()} />
            </div>
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
