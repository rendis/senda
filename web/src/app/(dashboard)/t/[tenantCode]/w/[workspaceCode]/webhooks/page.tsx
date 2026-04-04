import { PageShell } from "@/components/shared/page-shell";
import { WebhooksContent } from "@/components/webhooks/webhooks-content";

export default async function WorkspaceWebhooksPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;

  return (
    <PageShell
      title="Webhooks"
      description="Manage webhook endpoints for event notifications"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        { label: workspaceCode },
        { label: "Webhooks" },
      ]}
    >
      <WebhooksContent />
    </PageShell>
  );
}
