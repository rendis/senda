import { PageShell } from "@/components/shared/page-shell";
import { ApiKeysContent } from "@/components/api-keys/api-keys-content";

export default async function WorkspaceApiKeysPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;

  return (
    <PageShell
      title="API Keys"
      description="Manage API keys for programmatic access"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        { label: workspaceCode },
        { label: "API Keys" },
      ]}
    >
      <ApiKeysContent />
    </PageShell>
  );
}
