import { ExternalTemplateBuilderShell } from "@/components/templates/external-template-builder-shell";
import { normalizeEnvironment } from "@/lib/environment-mode";
import { SYSTEM_WORKSPACE_CODE, type ScopeContext } from "@/types/api";

interface ExternalTemplateBuilderPageProps {
  params: Promise<{
    profileSlug: string;
    environment: string;
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

export default async function ExternalTemplateBuilderEnvironmentPage({
  params,
  searchParams,
}: ExternalTemplateBuilderPageProps) {
  const { profileSlug, environment, tenantCode, workspaceCode, slug } =
    await params;
  const query = await searchParams;

  const scope: ScopeContext = {
    level: "workspace",
    tenantCode,
    workspaceCode,
    profileSlug,
    environment: normalizeEnvironment(environment),
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
