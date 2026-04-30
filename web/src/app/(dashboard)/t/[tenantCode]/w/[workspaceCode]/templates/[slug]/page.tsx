import { PageShell } from "@/components/shared/page-shell";
import { TemplatesListContent } from "@/components/templates/templates-list-content";
import { applyEnvironmentSearchParam } from "@/lib/environment-mode";

export default async function WorkspaceTemplatesListPage({
  params,
  searchParams,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
  searchParams: Promise<{ environment?: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const { environment } = await searchParams;

  return (
    <PageShell
      title="Templates"
      breadcrumbs={[
        { label: "Workspace" },
        {
          label: "Template Types",
          href: applyEnvironmentSearchParam(
            `/t/${tenantCode}/w/${workspaceCode}/templates`,
            environment,
          ),
        },
        { label: "Versions" },
      ]}
    >
      <TemplatesListContent />
    </PageShell>
  );
}
