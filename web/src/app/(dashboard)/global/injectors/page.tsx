import { PageShell } from "@/components/shared/page-shell";
import { InjectorsContent } from "@/components/injectors/injectors-content";

export default function GlobalInjectorsPage() {
  return (
    <PageShell
      title="Injectors"
      description="Template variable definitions and their resolved values across scopes"
      breadcrumbs={[{ label: "Global" }, { label: "Injectors" }]}
    >
      <InjectorsContent />
    </PageShell>
  );
}
