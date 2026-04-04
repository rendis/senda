import { PageShell } from "@/components/shared/page-shell";
import { DashboardContent } from "@/components/dashboard/dashboard-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

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
      description={`Workspace: ${workspaceCode}`}
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        ...(isSystem ? [] : [{ label: workspaceCode }]),
        { label: "Dashboard" },
      ]}
    >
      <DashboardContent />
    </PageShell>
  );
}
