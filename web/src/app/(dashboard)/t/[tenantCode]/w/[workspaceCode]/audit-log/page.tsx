import { PageShell } from "@/components/shared/page-shell";
import { AuditLogContent } from "@/components/audit-log/audit-log-content";

export default function WorkspaceAuditLogPage() {
  return (
    <PageShell
      title="Audit Log"
      breadcrumbs={[
        { label: "Workspace" },
        { label: "Audit Log" },
      ]}
    >
      <AuditLogContent />
    </PageShell>
  );
}
