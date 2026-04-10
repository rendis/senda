import { PageShell } from "@/components/shared/page-shell";
import { EmailDetailContent } from "@/components/emails/email-detail-content";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceEmailDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{
    tenantCode: string;
    workspaceCode: string;
    trackingId: string;
  }>;
  searchParams: Promise<{ environment?: string }>;
}) {
  const { tenantCode, workspaceCode, trackingId } = await params;
  const { environment } = await searchParams;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Email Detail"
      breadcrumbs={[
        {
          label: tenantCode,
          href: applyEnvironmentSearchParam(
            getTenantSystemPath(tenantCode),
            environment,
          ),
        },
        ...(isSystem
          ? []
          : [{
              label: getWorkspaceDisplayCode({ code: workspaceCode }),
              href: applyEnvironmentSearchParam(
                `/t/${tenantCode}/w/${workspaceCode}`,
                environment,
              ),
            }]),
        {
          label: "Emails",
          href: applyEnvironmentSearchParam(
            `/t/${tenantCode}/w/${workspaceCode}/emails`,
            environment,
          ),
        },
        { label: trackingId },
      ]}
    >
      <EmailDetailContent />
    </PageShell>
  );
}
