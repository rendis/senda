import { PageShell } from "@/components/shared/page-shell";
import { DashboardContent } from "@/components/dashboard/dashboard-content";

export default function GlobalDashboardPage() {
  return (
    <PageShell
      title="Dashboard"
      description="Global overview of all tenants and workspaces"
      breadcrumbs={[{ label: "Global" }, { label: "Dashboard" }]}
    >
      <DashboardContent />
    </PageShell>
  );
}
