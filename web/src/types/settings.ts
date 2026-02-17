/** System settings (global scope, superadmin only) */
export interface SystemSettings {
  oidc: OidcSettings;
  email_defaults: EmailDefaultSettings;
  alerts: AlertSettings;
  domain: DomainSettings;
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

export interface DomainSettings {
  recheck_interval_hours: number;
}

/** Update settings request (partial) */
export interface UpdateSettingsRequest {
  email_defaults?: Partial<EmailDefaultSettings>;
  alerts?: Partial<AlertSettings>;
  domain?: Partial<DomainSettings>;
}
