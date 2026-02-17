import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";

export default function GlobalSettingsPage() {
  return (
    <PageShell
      title="Settings"
      breadcrumbs={[{ label: "Global" }, { label: "Settings" }]}
    >
      <SettingsContent />
    </PageShell>
  );
}
