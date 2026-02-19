"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Languages } from "lucide-react";
import { LOCALE_COOKIE } from "@/lib/locale";
import type { SupportedLocale } from "@/lib/locale";

export function LocaleSwitcher() {
  const locale = useLocale() as SupportedLocale;
  const t = useTranslations("locale");
  const router = useRouter();

  function switchLocale() {
    const next: SupportedLocale = locale === "en" ? "es" : "en";
    document.cookie = `${LOCALE_COOKIE}=${next}; path=/; max-age=${60 * 60 * 24 * 365}; samesite=lax`;
    router.refresh();
  }

  return (
    <button
      onClick={switchLocale}
      className="flex items-center gap-2 w-full text-left"
    >
      <Languages className="h-4 w-4" />
      {t("switchTo")}
    </button>
  );
}
