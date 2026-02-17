import { PageShell } from "@/components/shared/page-shell";
import { DomainsContent } from "@/components/domains/domains-content";

export default function WorkspaceDomainsPage() {
  return (
    <PageShell
      title="Domains"
      description="Sending domains with DKIM verification status"
      breadcrumbs={[{ label: "Workspace" }, { label: "Domains" }]}
    >
      <DomainsContent />
    </PageShell>
  );
}
