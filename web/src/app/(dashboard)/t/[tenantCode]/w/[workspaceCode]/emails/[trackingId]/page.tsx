import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

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
        { label: tenantCode, href: getTenantSystemPath(tenantCode) },
        ...(isSystem ? [] : [{ label: getWorkspaceDisplayCode({ code: workspaceCode }) }]),
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
