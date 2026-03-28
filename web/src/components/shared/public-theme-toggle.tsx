"use client";

import { ThemeSelector } from "@/components/shared/theme-selector";

export function PublicThemeToggle() {
  return (
    <div className="fixed right-4 top-4 z-50 sm:right-6 sm:top-6">
      <ThemeSelector variant="inline" size="sm" showDescription={false} />
    </div>
  );
}
