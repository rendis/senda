import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

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
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Email Detail"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        ...(isSystem ? [] : [{ label: workspaceCode }]),
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
