import { PageShell } from "@/components/shared/page-shell";
import { WorkspacesContent } from "@/components/workspaces/workspaces-content";

export default async function WorkspaceLevelWorkspacesPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="Workspaces"
      description={`Manage workspace scopes inside tenant ${tenantCode}`}
      breadcrumbs={[{ label: tenantCode }, { label: "Workspaces" }]}
    >
      <WorkspacesContent />
    </PageShell>
  );
}
