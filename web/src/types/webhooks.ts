import type { WebhookStatus } from "./api";

/** Webhook event types */
export type WebhookEventType =
  | "email.sent"
  | "email.delivered"
  | "email.bounced"
  | "email.complained"
  | "email.opened";

/** Webhook record */
export interface Webhook {
  id: string;
  url: string;
  events: WebhookEventType[];
  is_active: boolean;
  consecutive_failures: number;
  secret?: string; // Only returned on creation
  created_at: string;
  updated_at: string;
}

/** Create webhook request */
export interface CreateWebhookRequest {
  url: string;
  events: WebhookEventType[];
}

/** Update webhook request */
export interface UpdateWebhookRequest {
  url?: string;
  events?: WebhookEventType[];
  is_active?: boolean;
}

/** Webhook test result */
export interface WebhookTestResult {
  id: string;
  test_delivery_id: string;
  status: string;
}
