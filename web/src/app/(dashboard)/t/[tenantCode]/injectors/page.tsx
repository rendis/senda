import { PageShell } from "@/components/shared/page-shell";
import { InjectorsContent } from "@/components/injectors/injectors-content";

export default function TenantInjectorsPage() {
  return (
    <PageShell
      title="Injectors"
      description="Template variable definitions and their resolved values across scopes"
      breadcrumbs={[{ label: "Tenant" }, { label: "Injectors" }]}
    >
      <InjectorsContent />
    </PageShell>
  );
}
