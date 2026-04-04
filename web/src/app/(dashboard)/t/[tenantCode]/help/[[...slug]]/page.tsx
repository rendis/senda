import { redirect } from "next/navigation";

export default async function TenantHelpPage({
  params,
}: {
  params: Promise<{ tenantCode: string; slug?: string[] }>;
}) {
  const { tenantCode, slug } = await params;
  const helpPath = slug?.length ? `/${slug.join("/")}` : "";
  redirect(`/t/${tenantCode}/w/_system/help${helpPath}`);
}
