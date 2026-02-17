import { PageShell } from "@/components/shared/page-shell";
import { EmailsContent } from "@/components/emails/emails-content";

export default function GlobalEmailsPage() {
  return (
    <PageShell
      title="Emails"
      description="Track and monitor all sent emails"
      breadcrumbs={[{ label: "Global" }, { label: "Emails" }]}
    >
      <EmailsContent />
    </PageShell>
  );
}
