import { PageShell } from "@/components/shared/page-shell";
import { DashboardContent } from "@/components/dashboard/dashboard-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceDashboardPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Dashboard"
      description={`Workspace: ${getWorkspaceDisplayCode({ code: workspaceCode })}`}
      breadcrumbs={[
        { label: tenantCode, href: getTenantSystemPath(tenantCode) },
        ...(isSystem ? [] : [{ label: getWorkspaceDisplayCode({ code: workspaceCode }) }]),
        { label: "Dashboard" },
      ]}
    >
      <DashboardContent />
    </PageShell>
  );
}
