/** System settings (global scope, superadmin only) */
export interface SystemSettings {
  oidc: OidcSettings;
  email_defaults: EmailDefaultSettings;
  alerts: AlertSettings;
}

/** Tenant-local workspace policies configured from the _system workspace */
export interface WorkspacePolicies {
  allow_workspace_local_templates: boolean;
  allow_workspace_inherited_template_forks: boolean;
  allow_workspace_local_injectors: boolean;
}

export interface OidcSettings {
  discovery_url: string;
  client_id: string;
  client_secret_set: boolean; // Never returns actual secret
}

export interface EmailDefaultSettings {
  max_retries: number;
  backoff_base_seconds: number;
  log_retention_days: number;
}

export interface AlertSettings {
  bounce_threshold_percent: number;
  complaint_threshold_percent: number;
}

/** Update settings request (partial) */
export interface UpdateSettingsRequest {
  email_defaults?: Partial<EmailDefaultSettings>;
  alerts?: Partial<AlertSettings>;
}

export type UpdateWorkspacePoliciesRequest = Partial<WorkspacePolicies>;
