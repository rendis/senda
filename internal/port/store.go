package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rendis/senda/internal/domain"
)

// TenantStore manages tenant persistence.
type TenantStore interface {
	Create(ctx context.Context, tenant *domain.Tenant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	GetByCode(ctx context.Context, code string) (*domain.Tenant, error)
	List(ctx context.Context, opts ListOptions) ([]*domain.Tenant, string, error)
	Update(ctx context.Context, tenant *domain.Tenant) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Purge(ctx context.Context, id uuid.UUID) error
}

// WorkspaceStore manages workspace persistence.
type WorkspaceStore interface {
	Create(ctx context.Context, ws *domain.Workspace) error
	CreateLogicalPair(ctx context.Context, prod *domain.Workspace, test *domain.Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
	GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID, environment domain.Environment) (*domain.Workspace, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, environment domain.Environment, opts ListOptions) ([]*domain.Workspace, string, error)
	UpdateShared(ctx context.Context, tenantID uuid.UUID, currentCode, nextCode, nextName string) error
	Update(ctx context.Context, ws *domain.Workspace) error
	SoftDeleteLogical(ctx context.Context, tenantID uuid.UUID, code string) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// GetUnsubscribeSigningKey returns the 32-byte HMAC key used to sign and
	// verify unsubscribe tokens for this workspace.
	GetUnsubscribeSigningKey(ctx context.Context, workspaceID uuid.UUID) ([]byte, error)
}

// WorkspaceExistenceStore provides batch existence checks for tenant-scoped workspace codes.
type WorkspaceExistenceStore interface {
	// ExistsActiveByTenantCode returns a dense map where each requested workspace code is present
	// and mapped to true only when the workspace exists for the tenant, is active, and is not soft-deleted.
	ExistsActiveByTenantCode(ctx context.Context, tenantCode string, workspaceCodes []string, environment domain.Environment) (map[string]bool, error)
}

// InjectorStore manages injector persistence.
type InjectorStore interface {
	// Definitions (schema)
	CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error
	UpdateDefinitionSchema(ctx context.Context, currentName string, workspaceID *uuid.UUID, def *domain.InjectorDefinition, fields []*domain.InjectorField) error
	GetDefinitionByID(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error)
	SoftDeleteDefinition(ctx context.Context, id uuid.UUID) error
	FindDefinitionByName(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error)
	ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)

	// Fields (immutable schema)
	CreateField(ctx context.Context, field *domain.InjectorField) error
	UpdateField(ctx context.Context, field *domain.InjectorField) error
	GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)

	// Values (overrideable)
	SetValue(ctx context.Context, val *domain.InjectorValue) error
	GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)

	// Batch operations (N+1 elimination)
	GetAllFieldsByDefinitions(ctx context.Context, defIDs []uuid.UUID) (map[uuid.UUID][]*domain.InjectorField, error)
	GetAllValuesByDefinitions(ctx context.Context, defIDs []uuid.UUID, chain []uuid.NullUUID) (map[uuid.UUID][]*domain.InjectorValue, error)
}

// TemplateTypeStore manages template type persistence.
type TemplateTypeStore interface {
	CreateType(ctx context.Context, tt *domain.TemplateType) error
	UpdateType(ctx context.Context, tt *domain.TemplateType) error
	SoftDeleteType(ctx context.Context, id uuid.UUID) error
	GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
	FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)
	ListTypes(ctx context.Context, wsID *uuid.UUID, opts ListOptions) ([]*domain.TemplateType, string, error)
}

// TemplateVersionStore manages template version persistence.
type TemplateVersionStore interface {
	CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error
	CloneVersion(ctx context.Context, templateID, sourceVersionID uuid.UUID, createdBy *uuid.UUID) (*domain.TemplateVersion, error)
	GetVersionByID(ctx context.Context, versionID uuid.UUID) (*domain.TemplateVersion, error)
	GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	UpdateVersion(ctx context.Context, ver *domain.TemplateVersion) error
	Publish(ctx context.Context, versionID uuid.UUID) error // archives previous published
	ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error)
	GetLatestVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	DeleteDraftVersion(ctx context.Context, versionID uuid.UUID) error
}

// LocaleStore manages template version locale persistence.
type LocaleStore interface {
	SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error
	GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
	ListLocales(ctx context.Context, versionID uuid.UUID) ([]*domain.TemplateVersionLocale, error)
	DeleteLocale(ctx context.Context, versionID uuid.UUID, locale string) error
}

// TemplateStore manages template persistence. It composes the narrower sub-interfaces
// so callers can accept a specific sub-interface when they don't need the full store.
type TemplateStore interface {
	TemplateTypeStore
	TemplateVersionStore
	LocaleStore

	// Core template methods
	CreateTemplate(ctx context.Context, tpl *domain.Template) error
	ForkTemplate(ctx context.Context, sourceTemplateID uuid.UUID, workspaceID uuid.UUID, createdBy *uuid.UUID) (*domain.Template, error)
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*domain.Template, error)
	GetTypeByID(ctx context.Context, id uuid.UUID) (*domain.TemplateType, error)
	GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error)
	ListByType(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID, opts ListOptions) ([]*domain.Template, string, error)
	ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error)
	SoftDeleteTemplate(ctx context.Context, templateID uuid.UUID) error
	SetDisabled(ctx context.Context, templateID uuid.UUID, wsID *uuid.UUID, disabled bool) error
}

// EmailHistoryType is one row of recipient history aggregated by template_type slug.
type EmailHistoryType struct {
	Slug       string
	LastSentAt time.Time
}

// EmailStore manages email persistence and queries.
type EmailStore interface {
	Create(ctx context.Context, email *domain.Email) error
	CreateTx(ctx context.Context, tx pgx.Tx, email *domain.Email) error
	GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error)
	GetByProviderMessageID(ctx context.Context, providerMessageID string) (*domain.Email, error)
	GetPayload(ctx context.Context, emailID uuid.UUID) (*domain.EmailPayload, error)
	PurgeWorkspaceRuntime(ctx context.Context, workspaceID uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.EmailStatus) error
	UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error
	SetProviderMessageID(ctx context.Context, id uuid.UUID, providerMessageID string) error

	AddEvent(ctx context.Context, event *domain.EmailEvent) error
	AddEventTx(ctx context.Context, tx pgx.Tx, event *domain.EmailEvent) error
	GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error)

	// Queries (scoped)
	QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
	QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error)
	QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters EmailFilters, cursor string, limit int) ([]*domain.Email, string, error)

	// DistinctTemplateTypesForRecipient returns one row per distinct template_type_slug
	// that the recipient received in the workspace since the given timestamp,
	// ordered by most recent first.
	DistinctTemplateTypesForRecipient(ctx context.Context, workspaceID uuid.UUID, email string, since time.Time) ([]EmailHistoryType, error)

	// Cross-tenant (superadmin only)
	QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
}

// MemberStore manages member persistence.
type MemberStore interface {
	Create(ctx context.Context, member *domain.Member) error
	GetByEmail(ctx context.Context, email string) (*domain.Member, error)
	GetByOIDCIdentity(ctx context.Context, issuer, subject string) (*domain.Member, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error)
	CountAll(ctx context.Context) (int64, error) // For onboarding check
	ListAll(ctx context.Context, opts ListOptions) ([]*domain.Member, string, error)
	ListInScope(ctx context.Context, scopeType domain.ScopeType, scopeID *uuid.UUID, opts ListOptions) ([]*domain.Member, string, error)

	AddRole(ctx context.Context, role *domain.MemberRole) error
	ReplaceRoleInScope(ctx context.Context, role *domain.MemberRole) error
	RemoveRole(ctx context.Context, roleID uuid.UUID) error
	RevokeAccessInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) (int64, error)
	GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
	GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error)

	// Batch operations (N+1 elimination)
	GetRolesByMembers(ctx context.Context, memberIDs []uuid.UUID) (map[uuid.UUID][]*domain.MemberRole, error)
}

// SuppressionStore manages suppression lists.
type SuppressionStore interface {
	// Global
	AddGlobal(ctx context.Context, entry *domain.SuppressionGlobal) error
	IsGloballySuppressed(ctx context.Context, email string) (bool, error)
	RemoveGlobal(ctx context.Context, email string, removedBy uuid.UUID, reason string) error

	// Workspace
	AddWorkspace(ctx context.Context, entry *domain.SuppressionWorkspace) error
	IsWorkspaceSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, error)

	// GetActiveWorkspaceSuppression returns the active (removed_at IS NULL) workspace
	// suppression for (workspaceID, email), or apperr.NotFound if none.
	GetActiveWorkspaceSuppression(ctx context.Context, workspaceID uuid.UUID, email string) (*domain.SuppressionWorkspace, error)

	// RemoveWorkspaceSuppression sets removed_at and removal_reason on the active
	// row for (workspaceID, email). Returns apperr.NotFound if no active row exists
	// (caller may treat as no-op for idempotent resubscribe flows).
	RemoveWorkspaceSuppression(ctx context.Context, workspaceID uuid.UUID, email string, removalReason string) error

	// Combined check (optimized)
	IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) // returns (suppressed, reason, err)

	// Batch combined check. Returned map contains only suppressed recipients keyed by normalized email.
	CheckBatch(ctx context.Context, wsID uuid.UUID, emails []string) (map[string]string, error)
}

// SuppressionStatus captures whether an email is suppressed and why.
type SuppressionStatus struct {
	Suppressed bool
	Reason     string
}

// AdapterStore manages email adapter persistence.
type AdapterStore interface {
	Create(ctx context.Context, adapter *domain.Adapter) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Adapter, error)
	Update(ctx context.Context, adapter *domain.Adapter) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// ListInChain returns all adapters visible in the resolution chain.
	ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Adapter, error)

	// ListByWorkspace returns adapters owned by a specific workspace (or global if nil).
	ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts ListOptions) (*PageResult[domain.Adapter], error)
}

// AdapterIdentityStore manages adapter identity persistence.
type AdapterIdentityStore interface {
	Create(ctx context.Context, identity *domain.AdapterIdentity) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.AdapterIdentity, error)
	Update(ctx context.Context, identity *domain.AdapterIdentity) error
	Delete(ctx context.Context, id uuid.UUID) error // hard delete

	ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterIdentity, error)
	GetDefault(ctx context.Context, adapterID uuid.UUID) (*domain.AdapterIdentity, error)
	SetDefault(ctx context.Context, adapterID uuid.UUID, identityID uuid.UUID) error
	UpsertBatch(ctx context.Context, adapterID uuid.UUID, identities []*domain.AdapterIdentity) error
	DeleteStale(ctx context.Context, adapterID uuid.UUID, keepIdentities []string) error
}

// AdapterGrantStore manages workspace visibility for system-owned adapters.
type AdapterGrantStore interface {
	ListAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID) ([]uuid.UUID, error)
	ReplaceAdapterWorkspaceGrants(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) error
	HasAdapterWorkspaceGrant(ctx context.Context, adapterID, workspaceID uuid.UUID) (bool, error)
	ListVisibleAdaptersForWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.Adapter], error)
}

// AdapterIdentityGrantStore manages workspace visibility for shared SES email identities.
type AdapterIdentityGrantStore interface {
	ListIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
	ReplaceIdentityWorkspaceGrants(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) error
	HasIdentityWorkspaceGrant(ctx context.Context, identityID, workspaceID uuid.UUID) (bool, error)
	ListGrantedIdentitiesForWorkspace(ctx context.Context, adapterID, workspaceID uuid.UUID) ([]*domain.AdapterIdentity, error)
}

// TemplateTypeUsageStore provides lightweight usage checks for shared access revocation.
type TemplateTypeUsageStore interface {
	ListWorkspacesUsingAdapter(ctx context.Context, adapterID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error)
	ListWorkspacesUsingSenderIdentity(ctx context.Context, identityID uuid.UUID, workspaceIDs []uuid.UUID) ([]uuid.UUID, error)
}

// WebhookStore manages webhook endpoint persistence.
type WebhookStore interface {
	Create(ctx context.Context, wh *domain.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	Update(ctx context.Context, wh *domain.Webhook) error
	Delete(ctx context.Context, id uuid.UUID) error // hard delete -- webhooks have no hierarchy

	// IncrementFailureCount atomically increments the failure counter and auto-disables after 10 consecutive failures.
	// Returns the new consecutive failure count and whether the webhook is still active.
	IncrementFailureCount(ctx context.Context, id uuid.UUID) (consecutiveFailures int, isActive bool, err error)

	// ResetFailureCount atomically resets the consecutive failure counter and last_failure_at on success.
	ResetFailureCount(ctx context.Context, id uuid.UUID) error

	// ListByWorkspace returns webhooks for a workspace.
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.Webhook], error)

	// GetActiveByWorkspace returns all enabled webhooks for event dispatch.
	GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error)
}

// SNSReplayDecision classifies the outcome of attempting to claim an SNS replay key.
type SNSReplayDecision string

const (
	SNSReplayDecisionAccepted  SNSReplayDecision = "accepted"
	SNSReplayDecisionDuplicate SNSReplayDecision = "duplicate"
	SNSReplayDecisionStale     SNSReplayDecision = "stale"
)

// SNSReplayStore persists SNS replay keys and enforces the replay window.
type SNSReplayStore interface {
	// Claim stores the replay key for the given SNS message if it is new and
	// inside the replay window. It returns a decision describing whether the
	// message was accepted, already seen, or stale.
	Claim(ctx context.Context, topicArn, messageID string, messageTimestamp time.Time, replayWindow time.Duration) (SNSReplayDecision, error)
}

// APIKeyStore manages API key persistence.
type APIKeyStore interface {
	Create(ctx context.Context, key *domain.APIKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	GetByHash(ctx context.Context, hash string) (*domain.APIKey, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	TouchLastUsed(ctx context.Context, id uuid.UUID) error

	// ListByWorkspace returns API keys for a workspace (hash excluded from response).
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.APIKey], error)
}

// AuditLogStore manages append-only audit log persistence.
type AuditLogStore interface {
	// Append writes a single audit entry. No update/delete allowed.
	Append(ctx context.Context, entry *domain.AuditLog) error

	// Query returns audit logs matching the filter with cursor pagination.
	Query(ctx context.Context, filter AuditFilter, opts ListOptions) (*PageResult[domain.AuditLog], error)
}

// GlobalConfigStore manages the singleton global configuration row.
type GlobalConfigStore interface {
	Get(ctx context.Context) (*domain.GlobalConfig, error)
	Upsert(ctx context.Context, cfg *domain.GlobalConfig) error
}

// ProvisioningStepStore manages adapter provisioning step persistence.
type ProvisioningStepStore interface {
	// InitSteps creates the provisioning step rows for an adapter (idempotent via ON CONFLICT DO NOTHING).
	InitSteps(ctx context.Context, adapterID uuid.UUID) error

	// InitDeprovisionSteps creates the deprovision step rows for an adapter (idempotent).
	InitDeprovisionSteps(ctx context.Context, adapterID uuid.UUID) error

	// ListByAdapter returns all provisioning steps for an adapter ordered by step_order.
	ListByAdapter(ctx context.Context, adapterID uuid.UUID) ([]*domain.AdapterProvisioningStep, error)

	// MarkCompleted sets a step to 'completed' with optional resource details.
	MarkCompleted(ctx context.Context, stepID uuid.UUID, resourceName, resourceARN *string) error

	// MarkFailed sets a step to 'failed' with an error message.
	MarkFailed(ctx context.Context, stepID uuid.UUID, errMsg string) error

	// ResetFailed resets all failed steps back to 'pending' for retry.
	ResetFailed(ctx context.Context, adapterID uuid.UUID) error

	// DeleteByAdapter removes all provisioning steps for an adapter.
	DeleteByAdapter(ctx context.Context, adapterID uuid.UUID) error
}

// TemplateTypeSubscriptionStore manages per-(workspace, template_type, email) subscription state.
type TemplateTypeSubscriptionStore interface {
	// Upsert inserts or updates the subscription state for (workspace, template_type, email).
	// ON CONFLICT (workspace_id, template_type_id, email) DO UPDATE.
	Upsert(ctx context.Context, sub *domain.TemplateTypeSubscription) error

	// GetState returns the current subscription row for the given key, or
	// a 404 AppError if none exists.
	GetState(ctx context.Context, workspaceID, templateTypeID uuid.UUID, email string) (*domain.TemplateTypeSubscription, error)

	// ListOptOutsForRecipient returns ALL rows (any subscribed value) for
	// (workspace, email). Used by the preference center to render current state.
	ListOptOutsForRecipient(ctx context.Context, workspaceID uuid.UUID, email string) ([]*domain.TemplateTypeSubscription, error)

	// BatchCheckOptOut returns the set of emails (from the input slice) that
	// are explicitly opted-OUT of the given template_type. Emails with no row
	// or with subscribed=true are NOT returned.
	BatchCheckOptOut(ctx context.Context, workspaceID, templateTypeID uuid.UUID, emails []string) (map[string]struct{}, error)
}

// ---- Pagination Types ----

// ListOptions for paginated list queries.
type ListOptions struct {
	Cursor string // opaque cursor (base64-encoded id+timestamp)
	Limit  int    // max items to return (default 25, max 100)
	Search string // ILIKE "%term%" on name/code (optional)
}

// PageResult wraps a paginated response.
type PageResult[T any] struct {
	Items      []*T   `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"` // empty = no more pages
	HasMore    bool   `json:"has_more"`
	Total      *int64 `json:"total,omitempty"` // only if explicitly requested
}

// ---- Filter Types ----

// EmailFilters for email query filtering.
type EmailFilters struct {
	ExternalID       *string
	Recipient        *string
	Status           *domain.EmailStatus
	TemplateTypeSlug *string
	AdapterID        *uuid.UUID
	Since            *time.Time
	Until            *time.Time
}

// AuditFilter for audit log query filtering.
type AuditFilter struct {
	TenantID    *uuid.UUID
	WorkspaceID *uuid.UUID
	ActorID     *uuid.UUID
	Action      *string
	EntityType  *string
	Since       *time.Time
	Until       *time.Time
}
