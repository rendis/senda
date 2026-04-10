import { redirect } from "next/navigation";
import { getTenantSystemPath } from "@/lib/system-workspace-display";

export default async function TenantTemplatesListPage({
  params,
}: {
  params: Promise<{ tenantCode: string; slug: string }>;
}) {
  const { tenantCode, slug } = await params;
  redirect(getTenantSystemPath(tenantCode, `/templates/${slug}`));
}
