import { PageShell } from "@/components/shared/page-shell";
import { TemplateTypesContent } from "@/components/templates/template-types-content";

export default function TenantTemplateTypesPage() {
  return (
    <PageShell
      title="Template Types"
      description="Manage email template types for this tenant"
      breadcrumbs={[{ label: "Tenant" }, { label: "Templates" }]}
    >
      <TemplateTypesContent />
    </PageShell>
  );
}
