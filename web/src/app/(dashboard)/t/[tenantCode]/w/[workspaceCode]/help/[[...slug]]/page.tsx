import { HelpArticle } from "@/components/help/help-article";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

export default async function WorkspaceHelpPage({
  params,
  searchParams,
}: {
  params: Promise<{
    tenantCode: string;
    workspaceCode: string;
    slug?: string[];
  }>;
  searchParams: Promise<{ environment?: string }>;
}) {
  const { tenantCode, workspaceCode, slug } = await params;
  const { environment } = await searchParams;
  const scopeLabel =
    workspaceCode === SYSTEM_WORKSPACE_CODE ? tenantCode : workspaceCode;

  return (
    <HelpArticle
      slug={slug}
      scopeLabel={scopeLabel}
      basePath={applyEnvironmentSearchParam(
        `/t/${tenantCode}/w/${workspaceCode}/help`,
        environment,
      )}
      tenantCode={tenantCode}
      workspaceCode={workspaceCode}
    />
  );
}
