import { PageShell } from "@/components/shared/page-shell";
import { EmailsContent } from "@/components/emails/emails-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

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
      description={`Workspace: ${workspaceCode}`}
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        ...(isSystem ? [] : [{ label: workspaceCode }]),
        { label: "Emails" },
      ]}
    >
      <EmailsContent />
    </PageShell>
  );
}
