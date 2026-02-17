import { PageShell } from "@/components/shared/page-shell";
import { MembersContent } from "@/components/members/members-content";

export default async function WorkspaceMembersPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;

  return (
    <PageShell
      title="Members"
      description="Manage team members and their roles"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}` },
        { label: workspaceCode },
        { label: "Members" },
      ]}
    >
      <MembersContent />
    </PageShell>
  );
}
