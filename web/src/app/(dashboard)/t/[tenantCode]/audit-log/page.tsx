import { redirect } from "next/navigation";

export default async function TenantAuditLogPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(`/t/${tenantCode}/w/_system/audit-log`);
}
