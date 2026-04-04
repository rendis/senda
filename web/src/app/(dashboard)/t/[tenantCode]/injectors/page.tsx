import { redirect } from "next/navigation";

export default async function TenantInjectorsPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;
  redirect(`/t/${tenantCode}/w/_system/injectors`);
}
