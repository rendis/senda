import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";

export default async function WorkspaceSettingsPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Settings"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        ...(isSystem ? [] : [{ label: workspaceCode }]),
        { label: "Settings" },
      ]}
    >
      <SettingsContent />
    </PageShell>
  );
}
