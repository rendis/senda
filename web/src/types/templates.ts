import type { TemplateVersionStatus, ScopeLevel } from "./api";

/** Template type (e.g., "welcome", "invoice") */
export interface TemplateType {
  id: string;
  slug: string;
  name: string;
  adapter_id?: string;
  variable_schema?: Record<string, unknown>;
  scope_level: ScopeLevel;
  created_at: string;
  updated_at: string;
}

/** Template (belongs to a template type at a scope level) */
export interface Template {
  id: string;
  template_type_id: string;
  is_disabled: boolean;
  scope_level: ScopeLevel;
  created_at: string;
  updated_at: string;
}

/** Template version */
export interface TemplateVersion {
  id: string;
  template_id: string;
  version_number: number;
  status: TemplateVersionStatus;
  subject: string;
  preview_text?: string;
  from_name: string;
  reply_to?: string;
  body_mjml: string;
  default_locale: string;
  editor_data?: Record<string, unknown>;
  created_by?: string;
  created_at: string;
  published_at?: string;
}

/** Template version locale override */
export interface TemplateLocale {
  id: string;
  version_id: string;
  locale: string;
  subject?: string;
  preview_text?: string;
  from_name?: string;
  body_mjml?: string;
  editor_data?: Record<string, unknown>;
}

/** Request to create a template version */
export interface CreateTemplateVersionRequest {
  subject: string;
  preview_text?: string;
  from_name: string;
  reply_to?: string;
  body_mjml: string;
  default_locale: string;
  editor_data?: Record<string, unknown>;
}

/** Request to send test email */
export interface TestSendRequest {
  recipient_email: string;
  variables?: Record<string, unknown>;
  locale?: string;
}

/** MJML preview response */
export interface MjmlPreviewResponse {
  html: string;
}
