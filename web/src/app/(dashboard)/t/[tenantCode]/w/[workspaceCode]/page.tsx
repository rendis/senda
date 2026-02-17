import { PageShell } from "@/components/shared/page-shell";

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
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border bg-card p-5">
          <p className="text-xs font-medium font-mono text-muted-foreground">Total Emails</p>
          <p className="text-[28px] font-semibold mt-2" style={{ letterSpacing: "-2px" }}>0</p>
          <p className="text-xs font-mono text-muted-foreground mt-2">No data yet</p>
        </div>
        <div className="rounded-lg border bg-card p-5">
          <p className="text-xs font-medium font-mono text-muted-foreground">Delivery Rate</p>
          <p className="text-[28px] font-semibold mt-2" style={{ letterSpacing: "-2px" }}>&mdash;</p>
          <p className="text-xs font-mono text-muted-foreground mt-2">No data yet</p>
        </div>
        <div className="rounded-lg border bg-card p-5">
          <p className="text-xs font-medium font-mono text-muted-foreground">Bounce Rate</p>
          <p className="text-[28px] font-semibold mt-2" style={{ letterSpacing: "-2px" }}>&mdash;</p>
          <p className="text-xs font-mono text-muted-foreground mt-2">No data yet</p>
        </div>
        <div className="rounded-lg border bg-card p-5">
          <p className="text-xs font-medium font-mono text-muted-foreground">Complaint Rate</p>
          <p className="text-[28px] font-semibold mt-2" style={{ letterSpacing: "-2px" }}>&mdash;</p>
          <p className="text-xs font-mono text-muted-foreground mt-2">No data yet</p>
        </div>
      </div>
    </PageShell>
  );
}
