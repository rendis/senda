/** Injector field type */
export type InjectorFieldType = "text" | "number" | "bool" | "img" | "url" | "html";

/** Injector field definition + runtime configuration */
export interface InjectorField {
  id?: string;
  field_name: string;
  field_type: InjectorFieldType;
  description?: string;
  position: number;
  default_value?: unknown;
  allow_overwrite: boolean;
}

/** Injector definition exposed to the UI */
export interface InjectorDefinition {
  id: string;
  workspace_id?: string;
  name: string;
  description?: string;
  fields: InjectorField[];
  created_at: string;
  updated_at: string;
}

export interface InjectorListResponse {
  items: InjectorDefinition[];
}

/** Request field for creating an injector */
export interface CreateInjectorFieldRequest {
  field_name: string;
  field_type: InjectorFieldType;
  description?: string;
  position: number;
  default_value?: unknown;
  allow_overwrite?: boolean;
}

/** Request body for POST /injectors */
export interface CreateInjectorRequest {
  name: string;
  description?: string;
  fields: CreateInjectorFieldRequest[];
}

/** Request body for PUT /injectors/:name */
export type UpdateInjectorRequest = CreateInjectorRequest;

/** Request body for PUT /injectors/:name/fields/:field_name */
export interface UpdateInjectorFieldRequest {
  default_value?: unknown;
  allow_overwrite?: boolean;
}
