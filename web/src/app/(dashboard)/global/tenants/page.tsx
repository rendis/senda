import { PageShell } from "@/components/shared/page-shell";
import { TenantsContent } from "@/components/tenants/tenants-content";

export default function GlobalTenantsPage() {
  return (
    <PageShell
      title="Tenants"
      description="Create, update, enable, disable, and retire tenant scopes across the platform"
      breadcrumbs={[{ label: "Global" }, { label: "Tenants" }]}
    >
      <TenantsContent />
    </PageShell>
  );
}
