import { getContext } from "@/lib/unsubscribe-api";
import { UnsubscribeForm } from "@/components/unsubscribe/unsubscribe-form";
import { getTranslations } from "next-intl/server";
import { Alert, AlertDescription } from "@/components/ui/alert";

export const dynamic = "force-dynamic";

export default async function UnsubscribePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const t = await getTranslations("unsubscribe");
  let ctx;
  try {
    ctx = await getContext(token);
  } catch {
    return (
      <Alert variant="destructive" data-testid="invalid-token-alert">
        <AlertDescription>{t("invalid_token")}</AlertDescription>
      </Alert>
    );
  }
  return <UnsubscribeForm token={token} initialContext={ctx} />;
}
