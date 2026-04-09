import type { ScopeLevel } from "@/types/api";
import type {
  CreateInjectorRequest,
  InjectorDefinition,
  InjectorField,
  InjectorFieldType,
} from "@/types/injectors";

export interface InjectorFormFieldEntry {
  field_name: string;
  field_type: InjectorFieldType;
  description: string;
  default_value: string;
  allow_overwrite: boolean;
}

export interface InjectorFormValues {
  name: string;
  description: string;
  fields: InjectorFormFieldEntry[];
}

export function emptyInjectorFormField(): InjectorFormFieldEntry {
  return {
    field_name: "",
    field_type: "text",
    description: "",
    default_value: "",
    allow_overwrite: true,
  };
}

export function injectorDefinitionToFormValues(
  injector?: Pick<InjectorDefinition, "name" | "description" | "fields">,
): InjectorFormValues {
  if (!injector) {
    return {
      name: "",
      description: "",
      fields: [emptyInjectorFormField()],
    };
  }

  const fields = [...injector.fields]
    .sort((left, right) => left.position - right.position)
    .map((field) => ({
      field_name: field.field_name,
      field_type: field.field_type,
      description: field.description ?? "",
      default_value: serializeInjectorFieldValue(field.field_type, field.default_value),
      allow_overwrite: field.allow_overwrite,
    }));

  return {
    name: injector.name,
    description: injector.description ?? "",
    fields: fields.length > 0 ? fields : [emptyInjectorFormField()],
  };
}

export function buildInjectorRequest(values: InjectorFormValues): CreateInjectorRequest {
  return {
    name: values.name,
    description: values.description.trim() ? values.description : undefined,
    fields: values.fields.map((field, index) => ({
      field_name: field.field_name,
      field_type: field.field_type,
      description: field.description.trim() ? field.description : undefined,
      position: index,
      default_value: parseInjectorFieldValue(field.field_type, field.default_value),
      allow_overwrite: field.allow_overwrite,
    })),
  };
}

export function canEditInjectorSchema(
  scopeLevel: ScopeLevel,
  injector?: Pick<InjectorDefinition, "workspace_id">,
): boolean {
  if (!injector) {
    return false;
  }

  if (scopeLevel === "global") {
    return !injector.workspace_id;
  }

  if (scopeLevel === "workspace") {
    return Boolean(injector.workspace_id);
  }

  return false;
}

export function supportsInjectorManagementScope(scopeLevel: ScopeLevel): boolean {
  return scopeLevel === "global" || scopeLevel === "workspace";
}

export function resolveUpdatedInjectorSelection(updated: Pick<InjectorDefinition, "name">): string {
  return updated.name;
}

export function serializeInjectorFieldValue(
  fieldType: InjectorFieldType,
  value: InjectorField["default_value"],
): string {
  if (value == null) {
    return fieldType === "bool" ? "false" : "";
  }

  if (fieldType === "bool") {
    return String(Boolean(value));
  }

  return String(value);
}

export function parseInjectorFieldValue(
  fieldType: InjectorFieldType,
  rawValue: string,
): unknown {
  if (fieldType === "bool") {
    return rawValue === "true";
  }

  if (fieldType === "number") {
    return rawValue === "" ? undefined : Number(rawValue);
  }

  return rawValue === "" ? undefined : rawValue;
}

export function isEmptyInjectorFieldDefault(field: InjectorFormFieldEntry): boolean {
  if (field.field_type === "bool") {
    return false;
  }

  return field.default_value.trim() === "";
}
