import { redirect } from "next/navigation";
import { getTenantSystemPath } from "@/lib/system-workspace-display";

export default async function TenantHelpPage({
  params,
}: {
  params: Promise<{ tenantCode: string; slug?: string[] }>;
}) {
  const { tenantCode, slug } = await params;
  const helpPath = slug?.length ? `/${slug.join("/")}` : "";
  redirect(getTenantSystemPath(tenantCode, `/help${helpPath}`));
}
