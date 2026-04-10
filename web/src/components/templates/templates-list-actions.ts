import type { TemplateVersionStatus } from "@/types/api";

export interface VersionPrimaryAction {
  icon: "pencil" | "file-text";
  labelKey: "editVersion" | "openVersion";
}

export function getVersionPrimaryAction(
  status: TemplateVersionStatus,
  draftAction: "edit" | "open" = "edit",
): VersionPrimaryAction {
  if (status === "draft") {
    return {
      icon: draftAction === "edit" ? "pencil" : "file-text",
      labelKey: draftAction === "edit" ? "editVersion" : "openVersion",
    };
  }

  return {
    icon: "file-text",
    labelKey: "openVersion",
  };
}
