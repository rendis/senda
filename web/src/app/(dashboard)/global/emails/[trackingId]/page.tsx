import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";

export default async function GlobalEmailDetailPage({
  params,
}: {
  params: Promise<{ trackingId: string }>;
}) {
  const { trackingId } = await params;

  return (
    <PageShell
      title="Email Detail"
      breadcrumbs={[
        { label: "Global" },
        { label: "Emails", href: "/global/emails" },
        { label: trackingId },
      ]}
    >
      <EmailDetailContent />
    </PageShell>
  );
}
