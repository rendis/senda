import type { DomainStatus } from "./api";

/** DNS record for domain verification */
export interface DnsRecord {
  type: string;
  name: string;
  value: string;
}

/** Domain record */
export interface Domain {
  id: string;
  domain_name: string;
  status: DomainStatus;
  dkim_selector?: string;
  dkim_public_key?: string;
  dns_records: DnsRecord[];
  verified_at?: string;
  last_check_at?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

/** Register domain request */
export interface RegisterDomainRequest {
  domain_name: string;
}
