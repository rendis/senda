import { PageShell } from "@/components/shared/page-shell";
import { EmailsContent } from "@/components/emails/emails-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceEmailsPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Emails"
      description={`Workspace: ${getWorkspaceDisplayCode({ code: workspaceCode })}`}
      breadcrumbs={[
        { label: tenantCode, href: getTenantSystemPath(tenantCode) },
        ...(isSystem ? [] : [{ label: getWorkspaceDisplayCode({ code: workspaceCode }) }]),
        { label: "Emails" },
      ]}
    >
      <EmailsContent />
    </PageShell>
  );
}
