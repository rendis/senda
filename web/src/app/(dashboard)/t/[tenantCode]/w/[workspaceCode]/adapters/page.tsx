import { PageShell } from "@/components/shared/page-shell";
import { AdaptersContent } from "@/components/adapters/adapters-content";

export default function WorkspaceAdaptersPage() {
  return (
    <PageShell
      title="Adapters"
      description="Email sending adapters (SES, Gmail) configured for this scope"
      breadcrumbs={[{ label: "Workspace" }, { label: "Adapters" }]}
    >
      <AdaptersContent />
    </PageShell>
  );
}
