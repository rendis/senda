import { redirect } from "next/navigation";

export default async function TenantEmailDetailPage({
  params,
}: {
  params: Promise<{ tenantCode: string; trackingId: string }>;
}) {
  const { tenantCode, trackingId } = await params;
  redirect(`/t/${tenantCode}/w/_system/emails/${trackingId}`);
}
