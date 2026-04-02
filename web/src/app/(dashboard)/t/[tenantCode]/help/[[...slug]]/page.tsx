import { HelpArticle } from "@/components/help/help-article";

export default async function TenantHelpPage({
  params,
}: {
  params: Promise<{ tenantCode: string; slug?: string[] }>;
}) {
  const { tenantCode, slug } = await params;

  return (
    <HelpArticle
      slug={slug}
      scopeLabel={tenantCode}
      basePath={`/t/${tenantCode}/help`}
    />
  );
}
