import { PageShell } from "@/components/shared/page-shell";
import { EmailsContent } from "@/components/emails/emails-content";

export default async function TenantEmailsPage({
  params,
}: {
  params: Promise<{ tenantCode: string }>;
}) {
  const { tenantCode } = await params;

  return (
    <PageShell
      title="Emails"
      description={`Tenant: ${tenantCode}`}
      breadcrumbs={[{ label: tenantCode }, { label: "Emails" }]}
    >
      <EmailsContent />
    </PageShell>
  );
}
