import { ExternalTemplateBuilderShell } from "@/components/templates/external-template-builder-shell";
import { SYSTEM_WORKSPACE_CODE, type ScopeContext } from "@/types/api";

interface ExternalTemplateBuilderPageProps {
  params: Promise<{
    profileSlug: string;
    tenantCode: string;
    workspaceCode: string;
    slug: string;
  }>;
  searchParams: Promise<{
    templateId?: string;
    versionId?: string;
    fallback?: string;
    readonly?: string;
  }>;
}

export default async function ExternalTemplateBuilderPage({
  params,
  searchParams,
}: ExternalTemplateBuilderPageProps) {
  const { profileSlug, tenantCode, workspaceCode, slug } = await params;
  const query = await searchParams;

  const scope: ScopeContext = {
    level: "workspace",
    tenantCode,
    workspaceCode,
  };

  const fallbackToSystem =
    workspaceCode === SYSTEM_WORKSPACE_CODE ||
    query.fallback === "system" ||
    query.readonly === "1";

  return (
    <ExternalTemplateBuilderShell
      profileSlug={profileSlug}
      templateSlug={slug}
      scope={scope}
      fallbackToSystem={fallbackToSystem}
    />
  );
}
