import type { InjectorDefinition } from "@/types/injectors";
import type { TemplateInjectorUsage } from "./test-send-injector-usage";

export interface TestSendInjectorCatalogRequest {
  enabled: boolean;
  includeInherited: boolean;
}

export function resolveTestSendInjectorCatalogRequest(
  allowedInjectorUsage: TemplateInjectorUsage,
): TestSendInjectorCatalogRequest {
  const templateUsesInjectors = Object.keys(allowedInjectorUsage).length > 0;
  return {
    enabled: templateUsesInjectors,
    includeInherited: templateUsesInjectors,
  };
}

export function resolveVisibleTestSendInjectors(
  injectors: InjectorDefinition[],
  allowedInjectorUsage: TemplateInjectorUsage,
): InjectorDefinition[] {
  const allowedInjectorNames = Object.keys(allowedInjectorUsage);
  if (!allowedInjectorNames.length) {
    return [];
  }

  return injectors.reduce<InjectorDefinition[]>((acc, injector) => {
    const allowedFields = allowedInjectorUsage[injector.name];
    if (!allowedFields?.length) {
      return acc;
    }

    const filteredFields = (injector.fields ?? []).filter((field) =>
      allowedFields.includes(field.field_name),
    );
    if (!filteredFields.length) {
      return acc;
    }

    acc.push({
      ...injector,
      fields: filteredFields,
    });
    return acc;
  }, []);
}
