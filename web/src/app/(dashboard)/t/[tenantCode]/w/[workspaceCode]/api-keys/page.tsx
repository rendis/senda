import { PageShell } from "@/components/shared/page-shell";
import { ApiKeysContent } from "@/components/api-keys/api-keys-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceApiKeysPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="API Keys"
      description="Manage API keys for programmatic access"
      breadcrumbs={[
        { label: tenantCode, href: getTenantSystemPath(tenantCode) },
        ...(isSystem ? [] : [{ label: getWorkspaceDisplayCode({ code: workspaceCode }) }]),
        { label: "API Keys" },
      ]}
    >
      <ApiKeysContent />
    </PageShell>
  );
}
