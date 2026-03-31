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
	Locale         *string
}

// TestSendResult holds the result of a successful test send.
type TestSendResult struct {
	ProviderMessageID string
	FromAddress       string
}

// TestSendService handles synchronous test-send for templates.
// It orchestrates: template resolution → MJML compilation → variable rendering → direct send.
type TestSendService struct {
	templateStore port.TemplateStore
	adapterStore  port.AdapterStore
	identityStore port.AdapterIdentityStore
	crypto        port.Crypto
	compiler      port.TemplateCompiler
	renderer      port.VariableRenderer
	senderFactory port.SenderFactory
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
) *TestSendService {
	return &TestSendService{
		templateStore: templateStore,
		adapterStore:  adapterStore,
		identityStore: identityStore,
		crypto:        crypto,
		compiler:      compiler,
		renderer:      renderer,
		senderFactory: senderFactory,
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
	from := resolution.ResolveFromAddress(ctx, s.identityStore, adapter, decrypted)
	if from.Address == "" {
		return nil, fmt.Errorf("%w: no sender identity for adapter %s", domain.ErrNoDefaultIdentity, adapter.Name)
	}

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

	// 6. Compile MJML → HTML.
	bodyHTML, err := s.compiler.Compile(ctx, bodyMJML)
	if err != nil {
		return nil, fmt.Errorf("compile MJML: %w", err)
	}

	// 7. Render variables in subject and body.
	if req.Variables != nil {
		if rendered, err := s.renderer.Render(subject, nil, req.Variables); err == nil {
			subject = rendered
		}
		if rendered, err := s.renderer.Render(bodyHTML, nil, req.Variables); err == nil {
			bodyHTML = rendered
		}
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

