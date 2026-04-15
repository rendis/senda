package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/resolution"
)

type SendContextBuilder struct {
	templateResolver *resolution.TemplateResolver
	adapterResolver  *resolution.AdapterResolver
	accessService    *AdapterAccessService
	identitySvc      *IdentityService
	tenantStore      interface {
		GetByCode(ctx context.Context, code string) (*domain.Tenant, error)
	}
	wsStore interface {
		GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
		GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
	}
}

type PreparedSendContext struct {
	builder *SendContextBuilder

	ref       *domain.TemplateRef
	refRaw    string
	tenant    *domain.Tenant
	workspace *domain.Workspace
	source    SendSource

	executionCache map[string]*PreparedTemplateExecution
}

type PreparedTemplateExecution struct {
	Resolved  *resolution.ResolvedTemplate
	Adapter   *resolution.ResolvedAdapter
	FromEmail string
}

func NewSendContextBuilder(
	templateResolver *resolution.TemplateResolver,
	adapterResolver *resolution.AdapterResolver,
	identitySvc *IdentityService,
	tenantStore interface {
		GetByCode(ctx context.Context, code string) (*domain.Tenant, error)
	},
	wsStore interface {
		GetByID(ctx context.Context, id uuid.UUID) (*domain.Workspace, error)
		GetByTenantAndCode(ctx context.Context, tenantID uuid.UUID, code string, environment domain.Environment) (*domain.Workspace, error)
	},
) *SendContextBuilder {
	return &SendContextBuilder{
		templateResolver: templateResolver,
		adapterResolver:  adapterResolver,
		identitySvc:      identitySvc,
		tenantStore:      tenantStore,
		wsStore:          wsStore,
	}
}

func (b *SendContextBuilder) SetAdapterAccessService(accessService *AdapterAccessService) {
	b.accessService = accessService
}

func (b *SendContextBuilder) Prepare(
	ctx context.Context,
	refRaw string,
	authWorkspaceID uuid.UUID,
	source SendSource,
) (*PreparedSendContext, error) {
	ref, err := domain.ParseRef(refRaw)
	if err != nil {
		return nil, err
	}

	tenant, err := b.tenantStore.GetByCode(ctx, ref.TenantCode)
	if err != nil {
		return nil, err
	}

	environment := domain.EnvironmentProd
	if authWorkspaceID != uuid.Nil {
		authWorkspace, err := b.wsStore.GetByID(ctx, authWorkspaceID)
		if err != nil {
			return nil, err
		}
		environment = authWorkspace.Environment
		if !environment.Valid() {
			environment = domain.EnvironmentProd
		}
	}

	ws, err := b.wsStore.GetByTenantAndCode(ctx, tenant.ID, ref.WorkspaceCode, environment)
	if err != nil {
		return nil, err
	}

	if authWorkspaceID != uuid.Nil && ws.ID != authWorkspaceID {
		slog.Warn("send rejected", "reason", "scope_mismatch", "auth_workspace_id", authWorkspaceID, "ref_workspace_id", ws.ID)
		return nil, domain.ErrWorkspaceScopeMismatch
	}

	if ws.IsSystem {
		slog.Warn("send rejected", "reason", "system_workspace_blocked", "workspace_id", ws.ID)
		return nil, domain.ErrSystemWorkspaceBlocked
	}

	return &PreparedSendContext{
		builder:        b,
		ref:            ref,
		refRaw:         refRaw,
		tenant:         tenant,
		workspace:      ws,
		source:         effectiveSendSource(source),
		executionCache: make(map[string]*PreparedTemplateExecution),
	}, nil
}

func (p *PreparedSendContext) ResolveTemplateExecution(ctx context.Context, locale *string) (*PreparedTemplateExecution, error) {
	cacheKey, effectiveLocale := p.localeCacheKey(locale)
	if execution, ok := p.executionCache[cacheKey]; ok {
		return execution, nil
	}

	resolved, err := p.builder.templateResolver.Resolve(ctx, p.workspace.ID, p.ref.TemplateType, effectiveLocale)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateDisabled) {
			slog.Warn("send rejected", "reason", "template_disabled", "template_type", p.ref.TemplateType, "workspace_id", p.workspace.ID)
		}
		return nil, err
	}

	adapter, err := p.builder.adapterResolver.ResolveForTemplateType(ctx, resolved.TemplateType)
	if err != nil {
		return nil, err
	}

	if p.builder.accessService != nil {
		if err := p.builder.accessService.ValidateTemplateTypeSelection(ctx, p.workspace, resolved.TemplateType.AdapterID, resolved.TemplateType.SenderIdentityID); err != nil {
			return nil, err
		}
	}

	fromEmail, err := resolveFromEmail(p.builder.identitySvc.identityStore, ctx, adapter.Adapter, resolved.TemplateType.SenderIdentityID)
	if err != nil {
		return nil, err
	}

	execution := &PreparedTemplateExecution{
		Resolved:  resolved,
		Adapter:   adapter,
		FromEmail: fromEmail,
	}
	p.executionCache[cacheKey] = execution
	return execution, nil
}

func (p *PreparedSendContext) localeCacheKey(locale *string) (string, *string) {
	effectiveLocale := locale
	if (effectiveLocale == nil || strings.TrimSpace(*effectiveLocale) == "") && p.workspace.DefaultLocale != nil && strings.TrimSpace(*p.workspace.DefaultLocale) != "" {
		effectiveLocale = p.workspace.DefaultLocale
	}
	if effectiveLocale == nil || strings.TrimSpace(*effectiveLocale) == "" {
		return "__default__", nil
	}
	value := strings.TrimSpace(*effectiveLocale)
	return value, &value
}
