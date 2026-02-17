import { PageShell } from "@/components/shared/page-shell";
import { ApiKeysContent } from "@/components/api-keys/api-keys-content";

export default function GlobalApiKeysPage() {
  return (
    <PageShell
      title="API Keys"
      description="Manage API keys for programmatic access"
      breadcrumbs={[{ label: "Global" }, { label: "API Keys" }]}
    >
      <ApiKeysContent />
    </PageShell>
  );
}
