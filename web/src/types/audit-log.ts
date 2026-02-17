/** Audit log entry */
export interface AuditLogEntry {
  id: string;
  actor_id: string;
  actor_email?: string;
  action: string;
  entity_type: string;
  entity_id: string;
  changes?: AuditLogChanges;
  created_at: string;
}

/** JSON diff for audit log detail */
export interface AuditLogChanges {
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
}

/** Filters for audit log */
export interface AuditLogFilters {
  actor_id?: string;
  action?: string;
  entity_type?: string;
  since?: string;
  until?: string;
}
