import type { ScopeLevel } from "./api";

/** Injector field type */
export type InjectorFieldType = "text" | "number" | "bool" | "img" | "url" | "html";

/** Injector field definition */
export interface InjectorField {
  field_name: string;
  field_type: InjectorFieldType;
  description?: string;
  position: number;
}

/** Injector definition (schema) */
export interface InjectorDefinition {
  id: string;
  name: string;
  description?: string;
  scope_level: ScopeLevel;
  fields: InjectorField[];
  created_at: string;
  updated_at: string;
}

/** Value at a specific scope level */
export interface InjectorScopeValue {
  value: unknown;
  scope_level: ScopeLevel;
  set_by?: string;
}

/** Resolution chain for a single field (showing values at each level) */
export interface InjectorFieldResolution {
  field_name: string;
  effective_value: unknown;
  global_level?: unknown;
  tenant_level?: unknown;
  workspace_level?: unknown;
}

/** Injector with resolved values */
export interface InjectorWithValues extends InjectorDefinition {
  values: Record<string, InjectorFieldResolution>;
}

/** Request to set injector values at workspace level */
export interface SetInjectorValuesRequest {
  values: Record<string, unknown>;
}

/** Request field for creating an injector */
export interface CreateInjectorFieldRequest {
  field_name: string;
  field_type: InjectorFieldType;
  description?: string;
  position: number;
}

/** Request body for POST /injectors */
export interface CreateInjectorRequest {
  name: string;
  description?: string;
  fields: CreateInjectorFieldRequest[];
}
