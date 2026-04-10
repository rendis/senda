import { PageShell } from "@/components/shared/page-shell";
import { SettingsContent } from "@/components/settings/settings-content";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceSettingsPage({
  params,
  searchParams,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
  searchParams: Promise<{ environment?: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const { environment } = await searchParams;
  const isSystem = workspaceCode === SYSTEM_WORKSPACE_CODE;

  return (
    <PageShell
      title="Settings"
      breadcrumbs={[
        {
          label: tenantCode,
          href: applyEnvironmentSearchParam(
            getTenantSystemPath(tenantCode),
            environment,
          ),
        },
        ...(isSystem
          ? []
          : [{
              label: getWorkspaceDisplayCode({ code: workspaceCode }),
              href: applyEnvironmentSearchParam(
                `/t/${tenantCode}/w/${workspaceCode}`,
                environment,
              ),
            }]),
        { label: "Settings" },
      ]}
    >
      <SettingsContent />
    </PageShell>
  );
}
