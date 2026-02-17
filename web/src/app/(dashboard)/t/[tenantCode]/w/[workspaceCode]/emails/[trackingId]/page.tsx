import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";

export default async function WorkspaceEmailDetailPage({
  params,
}: {
  params: Promise<{
    tenantCode: string;
    workspaceCode: string;
    trackingId: string;
  }>;
}) {
  const { tenantCode, workspaceCode, trackingId } = await params;

  return (
    <PageShell
      title="Email Detail"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}` },
        { label: workspaceCode },
        {
          label: "Emails",
          href: `/t/${tenantCode}/w/${workspaceCode}/emails`,
        },
        { label: trackingId },
      ]}
    >
      <EmailDetailContent />
    </PageShell>
  );
}
