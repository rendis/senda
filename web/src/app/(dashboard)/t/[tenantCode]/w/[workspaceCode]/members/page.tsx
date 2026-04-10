import { PageShell } from "@/components/shared/page-shell";
import { MembersContent } from "@/components/members/members-content";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";
import { SYSTEM_WORKSPACE_CODE } from "@/types/api";
import { getTenantSystemPath, getWorkspaceDisplayCode } from "@/lib/system-workspace-display";

export default async function WorkspaceMembersPage({
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
      title="Members"
      description="Manage team members and their roles"
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
        { label: "Members" },
      ]}
    >
      <MembersContent />
    </PageShell>
  );
}
