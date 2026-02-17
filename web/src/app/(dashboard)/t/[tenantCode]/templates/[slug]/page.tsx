import { PageShell } from "@/components/shared/page-shell";
import { TemplatesListContent } from "@/components/templates/templates-list-content";

export default function TenantTemplatesListPage() {
  return (
    <PageShell
      title="Templates"
      breadcrumbs={[
        { label: "Tenant" },
        { label: "Template Types", href: "../templates" },
        { label: "Versions" },
      ]}
    >
      <TemplatesListContent />
    </PageShell>
  );
}
