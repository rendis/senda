import { redirect } from "next/navigation";

export default async function TenantWorkspacesPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(`/t/${tenantCode}/w/_system/workspaces`);
}
