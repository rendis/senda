import { PageShell } from "@/components/shared/page-shell";
import { MembersContent } from "@/components/members/members-content";

export default async function TenantMembersPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="Members"
      description="Manage team members and their roles"
      breadcrumbs={[{ label: tenantCode }, { label: "Members" }]}
    >
      <MembersContent />
    </PageShell>
  );
}
