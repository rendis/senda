import type { EmailStatus } from "./api";

/** Full email record from API */
export interface Email {
  id: string;
  tracking_id: string;
  external_id?: string;
  workspace_id: string;
  tenant_id: string;
  adapter_id: string;
  template_type_slug: string;
  template_ref: string;
  status: EmailStatus;
  recipient_email: string;
  from_name: string;
  from_email: string;
  subject_rendered: string;
  locale?: string | null;
  provider_message_id?: string;
  source_type: string;
  source_actor_member_id?: string;
  source_actor_email?: string;
  retry_count: number;
  max_retries: number;
  variables?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

/** Email event in the timeline */
export interface EmailEvent {
  id: string;
  email_id: string;
  event_type: EmailStatus;
  occurred_at: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

/** Email detail (includes events) */
export interface EmailDetail extends Email {
  events: EmailEvent[];
  body_mjml?: string;
  injectors_snapshot?: Record<string, unknown>;
}

/** Filters for email list */
export interface EmailFilters {
  status?: EmailStatus[];
  template_type?: string;
  adapter_id?: string;
  since?: string;
  until?: string;
  search?: string;
}
