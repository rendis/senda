import { redirect } from "next/navigation";

export default async function TenantMembersPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(`/t/${tenantCode}/w/_system/members`);
}
