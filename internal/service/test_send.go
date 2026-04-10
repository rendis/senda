package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
)

// TestSendRequest holds the parameters for a template test send.
type TestSendRequest struct {
	TemplateID     uuid.UUID
	RecipientEmail string
	Variables      map[string]any
	Injectors      map[string]map[string]any
	Locale         *string
	Headers        map[string]string
}

// TestSendResult holds the result of a successful test send.
type TestSendResult struct {
	ProviderMessageID string
	FromAddress       string
}

// TestSendService handles synchronous test-send for templates.
// It orchestrates: template resolution → MJML compilation → variable rendering → direct send.
type TestSendService struct {
	templateStore  port.TemplateStore
	adapterStore   port.AdapterStore
	identityStore  port.AdapterIdentityStore
	crypto         port.Crypto
	compiler       port.TemplateCompiler
	renderer       port.VariableRenderer
	senderFactory  port.SenderFactory
	injectorMerger *resolution.InjectorMerger
	tenantStore    port.TenantStore
	workspaceStore port.WorkspaceStore
}

// NewTestSendService creates a new TestSendService.
func NewTestSendService(
	templateStore port.TemplateStore,
	adapterStore port.AdapterStore,
	identityStore port.AdapterIdentityStore,
	crypto port.Crypto,
	compiler port.TemplateCompiler,
	renderer port.VariableRenderer,
	senderFactory port.SenderFactory,
	injectorMerger *resolution.InjectorMerger,
	tenantStore port.TenantStore,
	workspaceStore port.WorkspaceStore,
) *TestSendService {
	return &TestSendService{
		templateStore:  templateStore,
		adapterStore:   adapterStore,
		identityStore:  identityStore,
		crypto:         crypto,
		compiler:       compiler,
		renderer:       renderer,
		senderFactory:  senderFactory,
		injectorMerger: injectorMerger,
		tenantStore:    tenantStore,
		workspaceStore: workspaceStore,
	}
}

// Send resolves a template, compiles it, renders variables, and sends synchronously.
func (s *TestSendService) Send(ctx context.Context, req *TestSendRequest) (*TestSendResult, error) {
	// 1. Get version: prefer published, fall back to latest draft.
	ver, err := s.templateStore.GetPublishedVersion(ctx, req.TemplateID)
	if err != nil {
		latest, latestErr := s.templateStore.GetLatestVersion(ctx, req.TemplateID)
		if latestErr != nil {
			return nil, fmt.Errorf("%w: no versions available", domain.ErrNoPublishedVersion)
		}
		ver = latest
	}

	// 2. Resolve template → template type → adapter.
	tpl, err := s.templateStore.GetTemplateByID(ctx, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("resolve template: %w", err)
	}

	tt, err := s.templateStore.GetTypeByID(ctx, tpl.TemplateTypeID)
	if err != nil {
		return nil, fmt.Errorf("resolve template type: %w", err)
	}

	if tt.AdapterID == nil {
		return nil, fmt.Errorf("%w: no adapter assigned to template type %q", domain.ErrNoAdapterConfigured, tt.Slug)
	}

	adapter, err := s.adapterStore.GetByID(ctx, *tt.AdapterID)
	if err != nil {
		return nil, fmt.Errorf("resolve adapter: %w", err)
	}

	// 3. Create sender.
	decrypted, err := s.crypto.Decrypt(adapter.ConfigEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt adapter config: %w", err)
	}

	sender, err := s.senderFactory(ctx, adapter, decrypted)
	if err != nil {
		return nil, fmt.Errorf("create sender: %w", err)
	}

	// 4. Resolve from address.
	fromEmail, err := resolveFromEmailForTemplateTest(s.identityStore, ctx, adapter, decrypted, tt.SenderIdentityID)
	if err != nil {
		return nil, err
	}
	from := port.EmailAddress{Address: fromEmail}

	// 5. Resolve locale override if requested.
	bodyMJML := ver.BodyMJML
	subject := ver.Subject
	if req.Locale != nil && *req.Locale != "" {
		locale, err := s.templateStore.GetLocale(ctx, ver.ID, *req.Locale)
		if err == nil {
			if locale.BodyMJML != nil {
				bodyMJML = *locale.BodyMJML
			}
			if locale.Subject != nil {
				subject = *locale.Subject
			}
		}
	}

	injectors, err := s.resolveInjectors(ctx, tpl.WorkspaceID, tt.Slug, req)
	if err != nil {
		return nil, err
	}

	// 6. Compile MJML → HTML.
	bodyHTML, err := s.compiler.Compile(ctx, bodyMJML)
	if err != nil {
		return nil, fmt.Errorf("compile MJML: %w", err)
	}

	// 7. Render variables and injectors in subject and body.
	subject, err = s.renderer.Render(subject, injectors, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render subject: %w", err)
	}
	bodyHTML, err = s.renderer.Render(bodyHTML, injectors, req.Variables)
	if err != nil {
		return nil, fmt.Errorf("render body: %w", err)
	}

	// 8. Send synchronously with timeout.
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	msg := &port.OutgoingEmail{
		From:     from,
		To:       port.EmailAddress{Address: req.RecipientEmail},
		Subject:  subject,
		BodyHTML: bodyHTML,
		BodyText: bodyHTML,
	}

	providerMsgID, err := sender.Send(sendCtx, msg)
	if err != nil {
		return nil, fmt.Errorf("send email: %w", err)
	}

	return &TestSendResult{
		ProviderMessageID: providerMsgID,
		FromAddress:       from.Address,
	}, nil
}

func (s *TestSendService) resolveInjectors(
	ctx context.Context,
	workspaceID *uuid.UUID,
	templateTypeSlug string,
	req *TestSendRequest,
) (map[string]map[string]any, error) {
	if s.injectorMerger == nil {
		return nil, nil
	}
	if workspaceID == nil {
		injCtx := port.NewInjectorContext(req.Headers, "global::"+templateTypeSlug, req.Variables, uuid.Nil, uuid.Nil, domain.EnvironmentProd, templateTypeSlug)
		injCtx.SetRequestInjectors(req.Injectors)
		injectors, err := s.injectorMerger.ResolveGlobalWithContext(ctx, injCtx)
		if err != nil {
			return nil, fmt.Errorf("resolve injectors: %w", err)
		}
		return injectors, nil
	}
	if s.workspaceStore == nil {
		return nil, fmt.Errorf("resolve workspace: workspace store not configured")
	}
	workspace, err := s.workspaceStore.GetByID(ctx, *workspaceID)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	if s.tenantStore == nil {
		return nil, fmt.Errorf("resolve tenant: tenant store not configured")
	}
	tenant, err := s.tenantStore.GetByID(ctx, workspace.TenantID)
	if err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	ref := fmt.Sprintf("%s:%s:%s", tenant.Code, workspace.Code, templateTypeSlug)
	injCtx := port.NewInjectorContext(req.Headers, ref, req.Variables, tenant.ID, workspace.ID, workspace.Environment, templateTypeSlug)
	injCtx.SetRequestInjectors(req.Injectors)

	injectors, err := s.injectorMerger.ResolveWithContext(ctx, workspace.ID, injCtx)
	if err != nil {
		return nil, fmt.Errorf("resolve injectors: %w", err)
	}
	return injectors, nil
}
