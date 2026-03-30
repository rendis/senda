/** Adapter types */
export type AdapterType = "ses" | "gmail";

/** Adapter record */
export interface Adapter {
  id: string;
  name: string;
  adapter_type: AdapterType;
  is_default: boolean;
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
