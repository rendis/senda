import { redirect } from "next/navigation";

export default async function TenantSettingsPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(`/t/${tenantCode}/w/_system/settings`);
}
