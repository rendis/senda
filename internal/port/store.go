package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/senda-app/senda/internal/domain"
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
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
	GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.Workspace, error)
	GetSystemWorkspace(ctx context.Context, tenantID uuid.UUID) (*domain.Workspace, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID, opts ListOptions) ([]*domain.Workspace, string, error)
	Update(ctx context.Context, ws *domain.Workspace) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// InjectorStore manages injector persistence.
type InjectorStore interface {
	// Definitions (schema)
	CreateDefinition(ctx context.Context, def *domain.InjectorDefinition) error
	GetDefinitionByID(ctx context.Context, id uuid.UUID) (*domain.InjectorDefinition, error)
	FindDefinitionByName(ctx context.Context, name string, workspaceID *uuid.UUID) (*domain.InjectorDefinition, error)
	ListDefinitionsInChain(ctx context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error)

	// Fields (immutable schema)
	CreateField(ctx context.Context, field *domain.InjectorField) error
	GetFieldsByDefinition(ctx context.Context, defID uuid.UUID) ([]*domain.InjectorField, error)

	// Values (overrideable)
	SetValue(ctx context.Context, val *domain.InjectorValue) error
	GetValues(ctx context.Context, defID uuid.UUID, chain []uuid.NullUUID) ([]*domain.InjectorValue, error)
}

// TemplateStore manages template persistence.
type TemplateStore interface {
	// Types
	CreateType(ctx context.Context, tt *domain.TemplateType) error
	GetTypeBySlug(ctx context.Context, slug string, chain []uuid.NullUUID) (*domain.TemplateType, error)
	FindTypeBySlugInScope(ctx context.Context, slug string, wsID *uuid.UUID) (*domain.TemplateType, error)
	ListTypes(ctx context.Context, wsID *uuid.UUID, opts ListOptions) ([]*domain.TemplateType, string, error)

	// Templates
	CreateTemplate(ctx context.Context, tpl *domain.Template) error
	GetByTypeAndScope(ctx context.Context, typeID uuid.UUID, wsID *uuid.UUID) (*domain.Template, error)
	ResolveTemplate(ctx context.Context, typeID uuid.UUID, chain []uuid.NullUUID) (*domain.Template, error)

	// Versions
	CreateVersion(ctx context.Context, ver *domain.TemplateVersion) error
	GetPublishedVersion(ctx context.Context, templateID uuid.UUID) (*domain.TemplateVersion, error)
	Publish(ctx context.Context, versionID uuid.UUID) error // archives previous published
	ListVersions(ctx context.Context, templateID uuid.UUID) ([]*domain.TemplateVersion, error)

	// Locales
	SetLocale(ctx context.Context, locale *domain.TemplateVersionLocale) error
	GetLocale(ctx context.Context, versionID uuid.UUID, locale string) (*domain.TemplateVersionLocale, error)
}

// EmailStore manages email persistence and queries.
type EmailStore interface {
	Create(ctx context.Context, email *domain.Email) error
	GetByTrackingID(ctx context.Context, trackingID string) (*domain.Email, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EmailStatus) error
	UpdateRetry(ctx context.Context, id uuid.UUID, retryCount int, nextRetryAt *time.Time) error

	AddEvent(ctx context.Context, event *domain.EmailEvent) error
	GetEvents(ctx context.Context, emailID uuid.UUID) ([]*domain.EmailEvent, error)

	// Queries (scoped)
	QueryByExternalID(ctx context.Context, wsID uuid.UUID, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
	QueryByRecipient(ctx context.Context, wsID uuid.UUID, email string, cursor string, limit int) ([]*domain.Email, string, error)
	QueryByWorkspace(ctx context.Context, wsID uuid.UUID, filters EmailFilters, cursor string, limit int) ([]*domain.Email, string, error)

	// Cross-tenant (superadmin only)
	QueryByExternalIDGlobal(ctx context.Context, externalID string, cursor string, limit int) ([]*domain.Email, string, error)
}

// MemberStore manages member persistence.
type MemberStore interface {
	Create(ctx context.Context, member *domain.Member) error
	GetByEmail(ctx context.Context, email string) (*domain.Member, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Member, error)
	CountAll(ctx context.Context) (int64, error) // For onboarding check
	ListAll(ctx context.Context, opts ListOptions) ([]*domain.Member, string, error)

	AddRole(ctx context.Context, role *domain.MemberRole) error
	RemoveRole(ctx context.Context, roleID uuid.UUID) error
	GetRoles(ctx context.Context, memberID uuid.UUID) ([]*domain.MemberRole, error)
	GetRolesInScope(ctx context.Context, memberID uuid.UUID, scopeType domain.ScopeType, scopeID *uuid.UUID) ([]*domain.MemberRole, error)
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

	// Combined check (optimized)
	IsSuppressed(ctx context.Context, wsID uuid.UUID, email string) (bool, string, error) // returns (suppressed, reason, err)
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

// DomainStore manages domain persistence and verification state.
type DomainStore interface {
	Create(ctx context.Context, d *domain.Domain) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Domain, error)
	Update(ctx context.Context, d *domain.Domain) error
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// ListInChain returns all domains visible in the resolution chain.
	ListInChain(ctx context.Context, scopes []uuid.NullUUID) ([]*domain.Domain, error)

	// ListByWorkspace returns domains owned by a specific workspace (or global if nil).
	ListByWorkspace(ctx context.Context, workspaceID *uuid.UUID, opts ListOptions) (*PageResult[domain.Domain], error)

	// GetPendingVerifications returns domains needing DNS re-check.
	GetPendingVerifications(ctx context.Context, limit int) ([]*domain.Domain, error)
}

// WebhookStore manages webhook endpoint persistence.
type WebhookStore interface {
	Create(ctx context.Context, wh *domain.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Webhook, error)
	Update(ctx context.Context, wh *domain.Webhook) error
	Delete(ctx context.Context, id uuid.UUID) error // hard delete -- webhooks have no hierarchy

	// ListByWorkspace returns webhooks for a workspace.
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, opts ListOptions) (*PageResult[domain.Webhook], error)

	// GetActiveByWorkspace returns all enabled webhooks for event dispatch.
	GetActiveByWorkspace(ctx context.Context, workspaceID uuid.UUID) ([]*domain.Webhook, error)
}

// APIKeyStore manages API key persistence.
type APIKeyStore interface {
	Create(ctx context.Context, key *domain.APIKey) error
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
	Status         *domain.EmailStatus
	TemplateTypeSlug *string
	Since          *time.Time
	Until          *time.Time
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
