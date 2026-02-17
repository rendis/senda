import { PageShell } from "@/components/shared/page-shell";
import { MembersContent } from "@/components/members/members-content";

export default function GlobalMembersPage() {
  return (
    <PageShell
      title="Members"
      description="Manage team members and their roles"
      breadcrumbs={[{ label: "Global" }, { label: "Members" }]}
    >
      <MembersContent />
    </PageShell>
  );
}
