import { PageShell } from "@/components/shared/page-shell";
import { MembersContent } from "@/components/members/members-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

export default async function WorkspaceMembersPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Members"
      description="Manage team members and their roles"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        ...(isSystem ? [] : [{ label: workspaceCode }]),
        { label: "Members" },
      ]}
    >
      <MembersContent />
    </PageShell>
  );
}
