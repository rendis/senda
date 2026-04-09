import type { TemplateVersionStatus } from "@/types/api";

export interface VersionPrimaryAction {
  icon: "pencil" | "file-text";
  label: "Edit version" | "Open version";
}

export function getVersionPrimaryAction(
  status: TemplateVersionStatus,
): VersionPrimaryAction {
  if (status === "draft") {
    return {
      icon: "pencil",
      label: "Edit version",
    };
  }

  return {
    icon: "file-text",
    label: "Open version",
  };
}
