import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";

export default async function TenantEmailDetailPage({
  params,
}: {
  params: Promise<{ tenantCode: string; trackingId: string }>;
}) {
  const { tenantCode, trackingId } = await params;

  return (
    <PageShell
      title="Email Detail"
      breadcrumbs={[
        { label: tenantCode },
        { label: "Emails", href: `/t/${tenantCode}/emails` },
        { label: trackingId },
      ]}
    >
      <EmailDetailContent />
    </PageShell>
  );
}
