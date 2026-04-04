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

export const SYSTEM_WORKSPACE_CODE = "_system";

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
  is_active: boolean;
  delete_blocked_reason?: string;
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
  is_active: boolean;
  open_tracking_enabled: boolean;
  default_locale: string | null;
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

/** Onboarding setup response (summary types from backend) */
export interface OnboardingSetupResponse {
  member: { id: string; email: string };
  tenant: { id: string; code: string; name: string };
  workspace: { id: string; code: string; name: string };
}

/** Per-adapter totals for dashboard breakdown */
export interface DashboardAdapterTotals {
  adapter_id: string;
  adapter_name: string;
  adapter_type: string;
  sender_identity_id?: string;
  from_email: string;
  totals: DashboardTotals;
}

/** Dashboard stats response from backend */
export interface DashboardStats {
  totals: DashboardTotals;
  rates: DashboardRates;
  time_series: DashboardTimeSeriesPoint[];
  recent_emails: DashboardRecentEmail[];
  recent_activity: DashboardActivityItem[];
  by_adapter: DashboardAdapterTotals[];
}

export interface DashboardTotals {
  sent: number;
  delivered: number;
  bounced: number;
  complained: number;
  failed: number;
}

export interface DashboardRates {
  delivery_rate: number;
  bounce_rate: number;
  complaint_rate: number;
}

export interface DashboardTimeSeriesPoint {
  date: string;
  sent: number;
  delivered: number;
  bounced: number;
  failed: number;
}

export interface DashboardRecentEmail {
  id: string;
  tracking_id: string;
  recipient_email: string;
  template_type_slug: string;
  status: EmailStatus;
  created_at: string;
}

export interface DashboardActivityItem {
  id: string;
  actor_email: string;
  action: string;
  entity_type: string;
  created_at: string;
}
