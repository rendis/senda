import { PageShell } from "@/components/shared/page-shell";
import { DomainsContent } from "@/components/domains/domains-content";

export default function GlobalDomainsPage() {
  return (
    <PageShell
      title="Domains"
      description="Sending domains with DKIM verification status"
      breadcrumbs={[{ label: "Global" }, { label: "Domains" }]}
    >
      <DomainsContent />
    </PageShell>
  );
}
