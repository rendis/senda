import {
  Monitor,
  Moon,
  Sun,
  type LucideIcon,
} from "lucide-react";

export const THEME_MODES = ["light", "dark", "system"] as const;

export type ThemeMode = (typeof THEME_MODES)[number];

export const THEME_ICON_MAP: Record<ThemeMode, LucideIcon> = {
  light: Sun,
  dark: Moon,
  system: Monitor,
};
