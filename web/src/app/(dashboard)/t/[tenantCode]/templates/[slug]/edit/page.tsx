import { redirect } from "next/navigation";

export default async function TenantTemplateEditPage({
  params,
}: {
  params: Promise<{ tenantCode: string; slug: string }>;
}) {
  const { tenantCode, slug } = await params;
  redirect(`/t/${tenantCode}/w/_system/templates/${slug}/edit`);
}
