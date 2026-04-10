import { redirect } from "next/navigation";
import { getTenantSystemPath } from "@/lib/system-workspace-display";

export default async function TenantEmailDetailPage({
  params,
}: {
  params: Promise<{ tenantCode: string; trackingId: string }>;
}) {
  const { tenantCode, trackingId } = await params;
  redirect(getTenantSystemPath(tenantCode, `/emails/${trackingId}`));
}
