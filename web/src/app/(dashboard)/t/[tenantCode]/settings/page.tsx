import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";

export default function TenantSettingsPage() {
  return (
    <PageShell
      title="Settings"
      breadcrumbs={[{ label: "Tenant" }, { label: "Settings" }]}
    >
      <SettingsContent />
    </PageShell>
  );
}
