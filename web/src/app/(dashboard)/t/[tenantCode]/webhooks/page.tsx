import { PageShell } from "@/components/shared/page-shell";
import { WebhooksContent } from "@/components/webhooks/webhooks-content";

export default async function TenantWebhooksPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="Webhooks"
      description="Manage webhook endpoints for event notifications"
      breadcrumbs={[{ label: tenantCode }, { label: "Webhooks" }]}
    >
      <WebhooksContent />
    </PageShell>
  );
}
