import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Settings"
      breadcrumbs={[
        { label: tenantCode, href: getTenantSystemPath(tenantCode) },
        ...(isSystem ? [] : [{ label: getWorkspaceDisplayCode({ code: workspaceCode }) }]),
        { label: "Settings" },
      ]}
    >
      <SettingsContent />
    </PageShell>
  );
}
