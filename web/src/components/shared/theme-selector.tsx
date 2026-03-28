"use client";

import { useSyncExternalStore } from "react";
import { useTheme } from "next-themes";
import { useTranslations } from "next-intl";
import {
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { THEME_ICON_MAP, THEME_MODES, type ThemeMode } from "@/lib/theme";

interface ThemeSelectorProps {
  variant?: "inline" | "menu";
  className?: string;
  showDescription?: boolean;
  size?: "default" | "sm";
}

function normalizeTheme(theme?: string): ThemeMode {
  if (theme === "light" || theme === "dark" || theme === "system") {
    return theme;
  }
  return "system";
}

export function ThemeSelector({
  variant = "inline",
  className,
  showDescription = true,
  size = "default",
}: ThemeSelectorProps) {
  const tTheme = useTranslations("theme");
  const tAppearance = useTranslations("appearance");
  const { theme, resolvedTheme, setTheme } = useTheme();
  const mounted = useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );

  if (!mounted) {
    if (variant === "inline") {
      return (
        <div
          className={cn(
            "rounded-xl border border-border bg-card/80 p-1 shadow-sm",
            className,
          )}
        >
          <div className="flex items-center gap-1">
            {THEME_MODES.map((mode) => (
              <div
                key={mode}
                className={cn(
                  "animate-pulse rounded-lg bg-muted",
                  size === "sm" ? "h-8 w-20" : "h-9 w-24",
                )}
              />
            ))}
          </div>
        </div>
      );
    }
    return null;
  }

  const selectedTheme = normalizeTheme(theme);
  const activeTheme = normalizeTheme(resolvedTheme);

  if (variant === "menu") {
    return (
      <>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>{tTheme("label")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={selectedTheme}
          onValueChange={(value) => setTheme(value as ThemeMode)}
        >
          {THEME_MODES.map((mode) => {
            const Icon = THEME_ICON_MAP[mode];

            return (
              <DropdownMenuRadioItem key={mode} value={mode}>
                <Icon className="h-4 w-4" />
                {tTheme(mode)}
                {mode === "system" && (
                  <DropdownMenuShortcut>
                    {tTheme(activeTheme)}
                  </DropdownMenuShortcut>
                )}
              </DropdownMenuRadioItem>
            );
          })}
        </DropdownMenuRadioGroup>
      </>
    );
  }

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <div className="inline-flex rounded-xl border border-border bg-card/80 p-1 shadow-sm backdrop-blur-sm">
        {THEME_MODES.map((mode) => {
          const Icon = THEME_ICON_MAP[mode];
          const isSelected = selectedTheme === mode;

          return (
            <Button
              key={mode}
              type="button"
              variant={isSelected ? "secondary" : "ghost"}
              size={size === "sm" ? "sm" : "default"}
              onClick={() => setTheme(mode)}
              className={cn(
                "rounded-lg",
                size === "sm"
                  ? "h-8 gap-1.5 px-2.5 text-xs"
                  : "h-9 gap-2 px-3 text-sm",
                !isSelected && "text-muted-foreground",
              )}
              aria-pressed={isSelected}
              aria-label={`${tTheme("label")}: ${tTheme(mode)}`}
            >
              <Icon className={size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4"} />
              <span>{tTheme(mode)}</span>
            </Button>
          );
        })}
      </div>
      {showDescription && (
        <p className="text-xs text-muted-foreground">
          {tAppearance("localOnly")}{" "}
          {selectedTheme === "system"
            ? tTheme("followingSystem", { theme: tTheme(activeTheme) })
            : tTheme("current", { theme: tTheme(selectedTheme) })}
        </p>
      )}
    </div>
  );
}
