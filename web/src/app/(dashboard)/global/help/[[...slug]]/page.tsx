import { HelpArticle } from "@/components/help/help-article";

export default async function GlobalHelpPage({
  params,
}: {
  params: Promise<{ slug?: string[] }>;
}) {
  const { slug } = await params;

  return (
    <HelpArticle
      slug={slug}
      scopeLabel="Global"
      basePath="/global/help"
    />
  );
}
