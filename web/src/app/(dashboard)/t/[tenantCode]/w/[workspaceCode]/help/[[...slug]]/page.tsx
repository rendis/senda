import { HelpArticle } from "@/components/help/help-article";

export default async function WorkspaceHelpPage({
  params,
}: {
  params: Promise<{
    tenantCode: string;
    workspaceCode: string;
    slug?: string[];
  }>;
}) {
  const { tenantCode, workspaceCode, slug } = await params;

  return (
    <HelpArticle
      slug={slug}
      scopeLabel={workspaceCode}
      basePath={`/t/${tenantCode}/w/${workspaceCode}/help`}
      tenantCode={tenantCode}
      workspaceCode={workspaceCode}
    />
  );
}
