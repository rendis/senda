import { PageShell } from "@/components/shared/page-shell";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export default async function WorkspaceApiDocsPage({
  params,
}: {
  params: Promise<{ tenantCode: string; workspaceCode: string }>;
}) {
  const { tenantCode, workspaceCode } = await params;
  const referenceHref = `/t/${tenantCode}/w/${workspaceCode}/api-docs/reference`;

  return (
    <PageShell
      title="API Docs"
      description="Reference for services using workspace API Keys"
      breadcrumbs={[
        { label: tenantCode, href: `/t/${tenantCode}/w/_system` },
        { label: workspaceCode, href: `/t/${tenantCode}/w/${workspaceCode}` },
        { label: "API Docs" },
      ]}
    >
      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Authenticate with an API Key</CardTitle>
            <CardDescription>
              Workspace API keys authenticate server-to-server requests against the Senda data plane.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <p>
              Use the workspace-scoped key in the Authorization header:
            </p>
            <pre className="overflow-x-auto rounded-lg border bg-muted p-4 font-mono text-xs text-foreground">
{`Authorization: Bearer <api-key>`}
            </pre>
            <p>
              The reference below includes every endpoint available to workspace API key clients,
              with auth, params, request bodies, and responses.
            </p>
          </CardContent>
        </Card>

        <Card className="overflow-hidden">
          <CardContent className="p-0">
            <iframe
              title="Senda API Reference"
              src={referenceHref}
              className="h-[calc(100vh-21rem)] min-h-[720px] w-full border-0"
            />
          </CardContent>
        </Card>
      </div>
    </PageShell>
  );
}
