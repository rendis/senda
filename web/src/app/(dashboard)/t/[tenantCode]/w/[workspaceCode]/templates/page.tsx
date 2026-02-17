import { PageShell } from "@/components/shared/page-shell";
import { TemplateTypesContent } from "@/components/templates/template-types-content";

export default function WorkspaceTemplateTypesPage() {
  return (
    <PageShell
      title="Template Types"
      description="Manage email template types for this workspace"
      breadcrumbs={[{ label: "Workspace" }, { label: "Templates" }]}
    >
      <TemplateTypesContent />
    </PageShell>
  );
}
