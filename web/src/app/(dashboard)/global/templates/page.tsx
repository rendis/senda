import { PageShell } from "@/components/shared/page-shell";
import { TemplateTypesContent } from "@/components/templates/template-types-content";

export default function GlobalTemplateTypesPage() {
  return (
    <PageShell
      title="Template Types"
      description="Manage email template types and their configurations"
      breadcrumbs={[{ label: "Global", href: "/global" }, { label: "Templates" }]}
    >
      <TemplateTypesContent />
    </PageShell>
  );
}
