import { HelpArticle } from "@/components/help/help-article";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

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
  const scopeLabel =
    workspaceCode === SYSTEM_WORKSPACE_CODE ? tenantCode : workspaceCode;

  return (
    <HelpArticle
      slug={slug}
      scopeLabel={scopeLabel}
      basePath={`/t/${tenantCode}/w/${workspaceCode}/help`}
      tenantCode={tenantCode}
      workspaceCode={workspaceCode}
    />
  );
}
