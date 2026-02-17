import { PageShell } from "@/components/shared/page-shell";
import { EmailsContent } from "@/components/emails/emails-content";

export default async function WorkspaceEmailsPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;

  return (
    <PageShell
      title="Emails"
      description={`Workspace: ${workspaceCode}`}
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}` },
        { label: workspaceCode },
        { label: "Emails" },
      ]}
    >
      <EmailsContent />
    </PageShell>
  );
}
