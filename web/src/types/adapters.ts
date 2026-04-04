/** Adapter types */
export type AdapterType = "ses" | "gmail";

/** Adapter record */
export interface Adapter {
  id: string;
  workspace_id?: string;
  source_scope?: "workspace" | "system";
  source_workspace_id?: string;
  name: string;
  adapter_type: AdapterType;
  is_default: boolean;
  is_editable: boolean;
  is_shared: boolean;
  rate_limit_per_second?: number;
  config_meta?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

/** SES adapter config (for creation/update) */
export interface SesConfig {
  region: string;
  access_key_id: string;
  secret_access_key: string;
}

/** Gmail adapter config (for creation/update) — Service Account with delegation */
export interface GmailConfig {
  service_account_json: string;
  delegate_email: string;
}

/** Create adapter request */
export interface CreateAdapterRequest {
  name: string;
  adapter_type: AdapterType;
  config: SesConfig | GmailConfig;
  is_default?: boolean;
  rate_limit_per_second?: number;
}

/** Update adapter request */
export interface UpdateAdapterRequest {
  name?: string;
  config?: SesConfig | GmailConfig;
  is_default?: boolean;
  rate_limit_per_second?: number;
}

/** Adapter identity (verified sender email or domain) */
export interface AdapterIdentity {
  id: string;
  adapter_id: string;
  identity: string;
  identity_type: "email" | "domain";
  status: "verified" | "pending" | "failed";
  sending_enabled: boolean;
  is_default: boolean;
  display_name?: string;
  source: "provider" | "manual";
  last_synced_at?: string;
  created_at: string;
  updated_at: string;
}

export interface WorkspaceAccessItem {
  workspace_id: string;
  code: string;
  name: string;
  is_granted: boolean;
}

export interface WorkspaceAccessListResponse {
  items: WorkspaceAccessItem[];
}

/** Provisioning step status */
export type ProvisioningStepStatusType = "pending" | "completed" | "failed";

/** Overall provisioning status */
export type ProvisioningOverallStatus =
  | "not_started"
  | "in_progress"
  | "completed"
  | "failed";

/** Single provisioning step */
export interface ProvisioningStep {
  name: string;
  order: number;
  status: ProvisioningStepStatusType;
  resource_name?: string;
  resource_arn?: string;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
}

/** Provisioning status response */
export interface ProvisioningStatusResponse {
  adapter_id: string;
  status: ProvisioningOverallStatus;
  steps: ProvisioningStep[];
}

/** Provisioning step metadata — single source of truth for labels. */
export const PROVISIONING_STEPS: Record<string, { label: string; short: string }> = {
  create_configuration_set: { label: "Create Configuration Set", short: "Config Set" },
  create_sns_topic:         { label: "Create SNS Topic",         short: "SNS Topic" },
  create_event_destination: { label: "Configure Event Destination", short: "Event Dest" },
  subscribe_webhook:        { label: "Subscribe Webhook",        short: "Webhook" },
  save_configuration:       { label: "Save Configuration",       short: "Save Config" },
  verify_subscription:      { label: "Verify Subscription",      short: "Verify" },
};
