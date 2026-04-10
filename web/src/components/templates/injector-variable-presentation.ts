import type { OwnerScope } from "@/types/api";
import type { InjectorFieldType } from "@/types/injectors";

export type BuilderInjectorVariablePresentation = {
  fullLabel: string;
  fieldLabel: string;
  fieldType: InjectorFieldType;
  fieldDescription?: string;
  injectorDescription?: string;
  static?: boolean;
  inheritedFromSystem?: boolean;
  ownerScope?: OwnerScope;
  source?: "database" | "code";
};

export function injectorFieldTypeLabel(fieldType: InjectorFieldType): string {
  switch (fieldType) {
    case "bool":
      return "Boolean";
    case "img":
      return "Image";
    case "url":
      return "URL";
    case "html":
      return "HTML";
    case "number":
      return "Number";
    default:
      return "Text";
  }
}

export function builderInjectorStaticLabel(isStatic: boolean): string {
  return isStatic ? "Yes" : "No";
}

export function builderInjectorInheritedLabel(isInherited: boolean): string {
  return isInherited ? "Yes" : "No";
}

export function builderInjectorSourceLabel(source?: "database" | "code"): string {
  if (source === "code") {
    return "Code";
  }
  return "Database";
}

export function buildInjectorTooltipSections(
  variable: BuilderInjectorVariablePresentation,
) {
  return {
    name: variable.fullLabel,
    description: variable.fieldDescription?.trim() || undefined,
    injectorDescription: variable.injectorDescription?.trim() || undefined,
    details: [
      {
        label: "Static",
        value: builderInjectorStaticLabel(Boolean(variable.static)),
      },
      {
        label: "Type",
        value: injectorFieldTypeLabel(variable.fieldType),
      },
      {
        label: "Inherited",
        value: builderInjectorInheritedLabel(Boolean(variable.inheritedFromSystem)),
      },
      {
        label: "Source",
        value: builderInjectorSourceLabel(variable.source),
      },
    ],
  };
}

export function injectorFieldTypeIconName(fieldType: InjectorFieldType): string {
  switch (fieldType) {
    case "number":
      return "hash";
    case "bool":
      return "toggle-right";
    case "img":
      return "image";
    case "url":
      return "link";
    case "html":
      return "code-2";
    default:
      return "type";
  }
}
