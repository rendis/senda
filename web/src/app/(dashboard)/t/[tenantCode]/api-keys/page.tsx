import { PageShell } from "@/components/shared/page-shell";
import { ApiKeysContent } from "@/components/api-keys/api-keys-content";

export default async function TenantApiKeysPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="API Keys"
      description="Manage API keys for programmatic access"
      breadcrumbs={[{ label: tenantCode }, { label: "API Keys" }]}
    >
      <ApiKeysContent />
    </PageShell>
  );
}
