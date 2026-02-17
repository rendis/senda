/** API Key record (list view — full key never returned) */
export interface ApiKey {
  id: string;
  name: string;
  masked_key: string;
  created_at: string;
  last_used_at?: string;
}

/** API Key creation response (full key shown ONLY on creation) */
export interface ApiKeyCreateResponse {
  id: string;
  name: string;
  full_key: string;
  created_at: string;
}

/** Create API Key request */
export interface CreateApiKeyRequest {
  name: string;
}
