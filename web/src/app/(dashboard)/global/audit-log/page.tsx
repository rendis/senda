import { PageShell } from "@/components/shared/page-shell";
import { AuditLogContent } from "@/components/audit-log/audit-log-content";

export default function GlobalAuditLogPage() {
  return (
    <PageShell
      title="Audit Log"
      breadcrumbs={[{ label: "Global" }, { label: "Audit Log" }]}
    >
      <AuditLogContent />
    </PageShell>
  );
}
