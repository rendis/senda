/** System settings (global scope, superadmin only) */
export interface SystemSettings {
  oidc: OidcSettings;
  email_defaults: EmailDefaultSettings;
  alerts: AlertSettings;
  external_integrations?: ExternalIntegrationsSettings;
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

export interface ExternalIntegrationMethodDescriptor {
  name: string;
  description: string;
}

export interface ExternalIntegrationCapabilities {
  list_templates: boolean;
  view_versions: boolean;
  edit_versions: boolean;
  publish_versions: boolean;
  test_send: boolean;
  builder_access: boolean;
  metadata_access: boolean;
  locale_access: boolean;
}

export interface ExternalIntegrationProfile {
  slug: string;
  name: string;
  description: string;
  enabled: boolean;
  auth_method_name: string;
  resolver_name: string;
  allowed_origins: string[];
  allowed_headers: string[];
  required_headers: string[];
  capabilities: ExternalIntegrationCapabilities;
}

export interface ExternalIntegrationsSettings {
  profiles: ExternalIntegrationProfile[];
  available_auth_methods?: ExternalIntegrationMethodDescriptor[];
  available_resolvers?: ExternalIntegrationMethodDescriptor[];
}

/** Update settings request (partial) */
export interface UpdateSettingsRequest {
  email_defaults?: Partial<EmailDefaultSettings>;
  alerts?: Partial<AlertSettings>;
  external_integrations?: Partial<ExternalIntegrationsSettings>;
}

export type UpdateWorkspacePoliciesRequest = Partial<WorkspacePolicies>;
