import { PageShell } from "@/components/shared/page-shell";
import { AuditLogContent } from "@/components/audit-log/audit-log-content";

export default function TenantAuditLogPage() {
  return (
    <PageShell
      title="Audit Log"
      breadcrumbs={[{ label: "Tenant" }, { label: "Audit Log" }]}
    >
      <AuditLogContent />
    </PageShell>
  );
}
