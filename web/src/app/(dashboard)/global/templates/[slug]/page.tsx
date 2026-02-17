import { PageShell } from "@/components/shared/page-shell";
import { TemplatesListContent } from "@/components/templates/templates-list-content";

export default function GlobalTemplatesListPage() {
  return (
    <PageShell
      title="Templates"
      breadcrumbs={[
        { label: "Global", href: "/global" },
        { label: "Template Types", href: "/global/templates" },
        { label: "Versions" },
      ]}
    >
      <TemplatesListContent />
    </PageShell>
  );
}
