import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";

export default function WorkspaceSettingsPage() {
  return (
    <PageShell
      title="Settings"
      breadcrumbs={[{ label: "Workspace" }, { label: "Settings" }]}
    >
      <SettingsContent />
    </PageShell>
  );
}
