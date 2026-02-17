import { PageShell } from "@/components/shared/page-shell";
import { WebhooksContent } from "@/components/webhooks/webhooks-content";

export default function GlobalWebhooksPage() {
  return (
    <PageShell
      title="Webhooks"
      description="Manage webhook endpoints for event notifications"
      breadcrumbs={[{ label: "Global" }, { label: "Webhooks" }]}
    >
      <WebhooksContent />
    </PageShell>
  );
}
