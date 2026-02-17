/** Paginated response from Senda API */
export interface PaginatedResponse<T> {
  items: T[];
  next_cursor?: string;
  has_more: boolean;
}

/** Pagination query params */
export interface PaginationParams {
  cursor?: string;
  limit?: number;
}

/** Standard API error envelope */
export interface ApiError {
  error: {
    code: string;
    message: string;
    details?: FieldError[];
    request_id?: string;
  };
}

export interface FieldError {
  field: string;
  message: string;
}

/** Scope levels for the hierarchy */
export type ScopeLevel = "global" | "tenant" | "workspace";

/** Current scope context */
export interface ScopeContext {
  level: ScopeLevel;
  tenantCode?: string;
  workspaceCode?: string;
}

/** Email statuses */
export type EmailStatus =
  | "queued"
  | "processing"
  | "sent"
  | "delivered"
  | "opened"
  | "bounced"
  | "complained"
  | "failed"
  | "suppressed";

/** Template version statuses */
export type TemplateVersionStatus = "draft" | "published" | "archived";

/** Domain verification statuses */
export type DomainStatus = "pending" | "verified" | "error";

/** Webhook statuses */
export type WebhookStatus = "active" | "disabled";

/** RBAC roles */
export type Role =
  | "superadmin"
  | "tenant_admin"
  | "workspace_admin"
  | "workspace_editor"
  | "workspace_viewer";

/** Tenant */
export interface Tenant {
  id: string;
  code: string;
  name: string;
  created_at: string;
  updated_at: string;
}

/** Workspace */
export interface Workspace {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  is_system: boolean;
  open_tracking_enabled: boolean;
  default_locale: string;
  created_at: string;
  updated_at: string;
}

/** Member */
export interface Member {
  id: string;
  email: string;
  display_name: string;
  created_at: string;
  updated_at: string;
}

/** Member role assignment */
export interface MemberRole {
  id: string;
  member_id: string;
  role: Role;
  scope_type: "global" | "tenant" | "workspace";
  tenant_id?: string;
  workspace_id?: string;
  created_at: string;
}

/** Onboarding status */
export interface OnboardingStatus {
  needs_onboarding: boolean;
}

/** Onboarding setup response */
export interface OnboardingSetupResponse {
  member: Member;
  tenant: Tenant;
  workspace: Workspace;
}
