/** Adapter types */
export type AdapterType = "ses" | "gmail";

/** Adapter record */
export interface Adapter {
  id: string;
  name: string;
  adapter_type: AdapterType;
  is_default: boolean;
  rate_limit_per_second?: number;
  created_at: string;
  updated_at: string;
}

/** SES adapter config (for creation/update) */
export interface SesConfig {
  region: string;
  access_key_id: string;
  secret_access_key: string;
}

/** Gmail adapter config (for creation/update) */
export interface GmailConfig {
  oauth_client_id: string;
  oauth_client_secret: string;
  refresh_token: string;
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
