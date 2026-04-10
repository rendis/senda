import { redirect } from "next/navigation";
import { getTenantSystemPath } from "@/lib/system-workspace-display";

export default async function TenantApiKeysPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(getTenantSystemPath(tenantCode, "/api-keys"));
}
