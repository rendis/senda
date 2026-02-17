import { PageShell } from "@/components/shared/page-shell";
import { DashboardContent } from "@/components/dashboard/dashboard-content";

export default async function WorkspaceDashboardPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;

  return (
    <PageShell
      title="Dashboard"
      description={`Workspace: ${workspaceCode}`}
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}` },
        { label: workspaceCode },
        { label: "Dashboard" },
      ]}
    >
      <DashboardContent />
    </PageShell>
  );
}
