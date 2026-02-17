import { PageShell } from "@/components/shared/page-shell";
import { InjectorsContent } from "@/components/injectors/injectors-content";

export default function WorkspaceInjectorsPage() {
  return (
    <PageShell
      title="Injectors"
      description="Template variable definitions and their resolved values across scopes"
      breadcrumbs={[{ label: "Workspace" }, { label: "Injectors" }]}
    >
      <InjectorsContent />
    </PageShell>
  );
}
