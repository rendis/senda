import { PageShell } from "@/components/shared/page-shell";
import { DashboardContent } from "@/components/dashboard/dashboard-content";

export default async function TenantDashboardPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="Dashboard"
      description={`Tenant: ${tenantCode}`}
      breadcrumbs={[{ label: tenantCode }, { label: "Dashboard" }]}
    >
      <DashboardContent />
    </PageShell>
  );
}
