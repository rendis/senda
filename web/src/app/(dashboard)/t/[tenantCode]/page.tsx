import { redirect } from "next/navigation";
import { getTenantSystemPath } from "@/lib/system-workspace-display";

export default async function TenantRootPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(getTenantSystemPath(tenantCode));
}
