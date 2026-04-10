package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/service"
)

type mockCryptoTestSend struct{}

func (m *mockCryptoTestSend) Encrypt(plaintext []byte) ([]byte, error)  { return plaintext, nil }
func (m *mockCryptoTestSend) Decrypt(ciphertext []byte) ([]byte, error) { return ciphertext, nil }

type mockEmailSenderTestSend struct {
	sendFn func(ctx context.Context, msg *port.OutgoingEmail) (string, error)
}

func (m *mockEmailSenderTestSend) Send(ctx context.Context, msg *port.OutgoingEmail) (string, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return "provider-msg-id", nil
}

func (m *mockEmailSenderTestSend) Name() string { return "mock" }

func (m *mockEmailSenderTestSend) HealthCheck(_ context.Context) error { return nil }

type fixedCodeInjectorTestSend struct {
	code   string
	fields map[string]any
}

func (i fixedCodeInjectorTestSend) Code() string { return i.code }

func (i fixedCodeInjectorTestSend) Resolve() (port.CodeResolveFunc, []string) {
	return func(_ context.Context, _ *port.InjectorContext) (map[string]any, error) {
		out := make(map[string]any, len(i.fields))
		for key, value := range i.fields {
			out[key] = value
		}
		return out, nil
	}, nil
}

func (i fixedCodeInjectorTestSend) IsCritical() bool { return true }

func (i fixedCodeInjectorTestSend) Timeout() time.Duration { return 0 }

func TestTestSendService_ResolvesInjectorsWithRequestCodeAndDefaultPrecedence(t *testing.T) {
	tenantID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	templateTypeID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	templateStore := &mockTemplateStore{
		getPublishedVersionFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.TemplateVersion{
				ID:            uuid.Must(uuid.NewV7()),
				TemplateID:    templateID,
				VersionNumber: 1,
				Status:        domain.VersionStatusPublished,
				Subject:       "Student {{ injector.student.name }} {{ injector.student.age }} {{ injector.student.locked }}",
				BodyMJML:      "<mjml><mj-body><mj-section><mj-column><mj-text>Name={{ injector.student.name }} Age={{ injector.student.age }} Locked={{ injector.student.locked }} Event={{ event.user_name }}</mj-text></mj-column></mj-section></mj-body></mjml>",
				FromName:      "Support",
				DefaultLocale: "en",
			}, nil
		},
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: templateTypeID,
				WorkspaceID:    &workspaceID,
			}, nil
		},
		getTypeByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateType, error) {
			if id != templateTypeID {
				t.Fatalf("unexpected type id %s", id)
			}
			return &domain.TemplateType{
				ID:        templateTypeID,
				Slug:      "injector-runtime",
				Name:      "Injector Runtime",
				AdapterID: &adapterID,
			}, nil
		},
	}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.Adapter{
				ID:              adapterID,
				Name:            "SMTP Test",
				AdapterType:     domain.AdapterTypeSES,
				ConfigEncrypted: []byte(`{}`),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getDefaultFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.AdapterIdentity{
				ID:             uuid.Must(uuid.NewV7()),
				AdapterID:      adapterID,
				Identity:       "noreply@example.com",
				IdentityType:   domain.IdentityTypeEmail,
				Status:         domain.IdentityStatusVerified,
				SendingEnabled: true,
				IsDefault:      true,
				Source:         domain.IdentitySourceManual,
			}, nil
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id != workspaceID {
				t.Fatalf("unexpected workspace id %s", id)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: "main", Name: "Main"}, nil
		},
		getSystemWorkspaceFn: func(_ context.Context, gotTenantID uuid.UUID, _ domain.Environment) (*domain.Workspace, error) {
			if gotTenantID != tenantID {
				t.Fatalf("unexpected tenant id %s", gotTenantID)
			}
			return &domain.Workspace{ID: uuid.Must(uuid.NewV7()), TenantID: tenantID, Code: "_system", Name: "System", IsSystem: true}, nil
		},
	}

	tenantStore := &mockTenantStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Tenant, error) {
			if id != tenantID {
				t.Fatalf("unexpected tenant id %s", id)
			}
			return &domain.Tenant{ID: tenantID, Code: "acme", Name: "Acme"}, nil
		},
	}

	injectorDefinitionID := uuid.Must(uuid.NewV7())
	injectorStore := &mockInjectorStoreSend{
		listDefinitionsInChainFn: func(_ context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			if len(chain) != 2 || !chain[0].Valid || chain[0].UUID != workspaceID {
				t.Fatalf("unexpected chain %+v", chain)
			}
			return []*domain.InjectorDefinition{
				{
					ID:          injectorDefinitionID,
					Name:        "student",
					WorkspaceID: &workspaceID,
				},
			}, nil
		},
		getFieldsByDefinitionFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			if defID != injectorDefinitionID {
				t.Fatalf("unexpected definition id %s", defID)
			}
			return []*domain.InjectorField{
				{
					ID:                   uuid.Must(uuid.NewV7()),
					InjectorDefinitionID: injectorDefinitionID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					DefaultValue:         "Default Student",
					AllowOverwrite:       true,
				},
				{
					ID:                   uuid.Must(uuid.NewV7()),
					InjectorDefinitionID: injectorDefinitionID,
					FieldName:            "age",
					FieldType:            domain.FieldTypeNumber,
					DefaultValue:         11,
					AllowOverwrite:       true,
				},
				{
					ID:                   uuid.Must(uuid.NewV7()),
					InjectorDefinitionID: injectorDefinitionID,
					FieldName:            "locked",
					FieldType:            domain.FieldTypeText,
					DefaultValue:         "LOCKED-DEFAULT",
					AllowOverwrite:       false,
				},
			}, nil
		},
	}

	chainResolver := resolution.NewChainResolver(wsStore, newMockCacheSend())
	injectorMerger := resolution.NewInjectorMerger(
		injectorStore,
		chainResolver,
		nil,
		[]port.CodeInjector{
			fixedCodeInjectorTestSend{
				code: "student",
				fields: map[string]any{
					"name":   "Code Student",
					"age":    22,
					"locked": "CODE-SHOULD-NOT-WIN",
				},
			},
		},
		nil,
	)

	var captured *port.OutgoingEmail
	svc := service.NewTestSendService(
		templateStore,
		adapterStore,
		identityStore,
		&mockCryptoTestSend{},
		&mockTemplateCompiler{compileFn: func(_ context.Context, mjml string) (string, error) { return mjml, nil }},
		service.NewVariableRenderer(),
		func(ctx context.Context, adapter *domain.Adapter, decrypted []byte) (port.EmailSender, error) {
			return &mockEmailSenderTestSend{
				sendFn: func(_ context.Context, msg *port.OutgoingEmail) (string, error) {
					captured = msg
					return "provider-msg-id", nil
				},
			}, nil
		},
		injectorMerger,
		tenantStore,
		wsStore,
	)

	_, err := svc.Send(context.Background(), &service.TestSendRequest{
		TemplateID:     templateID,
		RecipientEmail: "student@example.com",
		Variables:      map[string]any{"user_name": "Event Ana"},
		Injectors: map[string]map[string]any{
			"student": {
				"name":   "Request Student",
				"locked": "REQUEST-SHOULD-NOT-WIN",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("expected outgoing email to be captured")
	}
	if captured.Subject != "Student Request Student 22 LOCKED-DEFAULT" {
		t.Fatalf("unexpected subject: %q", captured.Subject)
	}
	if captured.BodyHTML != "<mjml><mj-body><mj-section><mj-column><mj-text>Name=Request Student Age=22 Locked=LOCKED-DEFAULT Event=Event Ana</mj-text></mj-column></mj-section></mj-body></mjml>" {
		t.Fatalf("expected compiled html to be rendered through variable renderer, got %q", captured.BodyHTML)
	}
}

func TestTestSendService_ResolvesGlobalInjectorsWithRequestAndDefaultPrecedence(t *testing.T) {
	templateID := uuid.Must(uuid.NewV7())
	templateTypeID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	injectorDefinitionID := uuid.Must(uuid.NewV7())

	templateStore := &mockTemplateStore{
		getPublishedVersionFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.TemplateVersion{
				ID:            uuid.Must(uuid.NewV7()),
				TemplateID:    templateID,
				VersionNumber: 1,
				Status:        domain.VersionStatusPublished,
				Subject:       "Brand {{ injector.brand.name }} {{ injector.brand.locked }}",
				BodyMJML:      "<mjml><mj-body><mj-section><mj-column><mj-text>Name={{ injector.brand.name }} Locked={{ injector.brand.locked }}</mj-text></mj-column></mj-section></mj-body></mjml>",
				FromName:      "Support",
				DefaultLocale: "en",
			}, nil
		},
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: templateTypeID,
				WorkspaceID:    nil,
			}, nil
		},
		getTypeByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateType, error) {
			if id != templateTypeID {
				t.Fatalf("unexpected type id %s", id)
			}
			return &domain.TemplateType{
				ID:        templateTypeID,
				Slug:      "global-brand",
				Name:      "Global Brand",
				AdapterID: &adapterID,
			}, nil
		},
	}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.Adapter{
				ID:              adapterID,
				Name:            "SMTP Test",
				AdapterType:     domain.AdapterTypeSES,
				ConfigEncrypted: []byte(`{}`),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getDefaultFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.AdapterIdentity{
				ID:             uuid.Must(uuid.NewV7()),
				AdapterID:      adapterID,
				Identity:       "noreply@example.com",
				IdentityType:   domain.IdentityTypeEmail,
				Status:         domain.IdentityStatusVerified,
				SendingEnabled: true,
				IsDefault:      true,
				Source:         domain.IdentitySourceManual,
			}, nil
		},
	}

	injectorStore := &mockInjectorStoreSend{
		listDefinitionsInChainFn: func(_ context.Context, chain []uuid.NullUUID) ([]*domain.InjectorDefinition, error) {
			if len(chain) != 1 || chain[0].Valid {
				t.Fatalf("expected global-only chain, got %+v", chain)
			}
			return []*domain.InjectorDefinition{
				{
					ID:          injectorDefinitionID,
					Name:        "brand",
					WorkspaceID: nil,
				},
			}, nil
		},
		getFieldsByDefinitionFn: func(_ context.Context, defID uuid.UUID) ([]*domain.InjectorField, error) {
			if defID != injectorDefinitionID {
				t.Fatalf("unexpected definition id %s", defID)
			}
			return []*domain.InjectorField{
				{
					ID:                   uuid.Must(uuid.NewV7()),
					InjectorDefinitionID: injectorDefinitionID,
					FieldName:            "name",
					FieldType:            domain.FieldTypeText,
					DefaultValue:         "Global Brand",
					AllowOverwrite:       true,
				},
				{
					ID:                   uuid.Must(uuid.NewV7()),
					InjectorDefinitionID: injectorDefinitionID,
					FieldName:            "locked",
					FieldType:            domain.FieldTypeText,
					DefaultValue:         "LOCKED-GLOBAL",
					AllowOverwrite:       false,
				},
			}, nil
		},
	}

	injectorMerger := resolution.NewInjectorMerger(
		injectorStore,
		nil,
		nil,
		nil,
		nil,
	)

	var captured *port.OutgoingEmail
	svc := service.NewTestSendService(
		templateStore,
		adapterStore,
		identityStore,
		&mockCryptoTestSend{},
		&mockTemplateCompiler{compileFn: func(_ context.Context, mjml string) (string, error) { return mjml, nil }},
		service.NewVariableRenderer(),
		func(ctx context.Context, adapter *domain.Adapter, decrypted []byte) (port.EmailSender, error) {
			return &mockEmailSenderTestSend{
				sendFn: func(_ context.Context, msg *port.OutgoingEmail) (string, error) {
					captured = msg
					return "provider-msg-id", nil
				},
			}, nil
		},
		injectorMerger,
		nil,
		nil,
	)

	_, err := svc.Send(context.Background(), &service.TestSendRequest{
		TemplateID:     templateID,
		RecipientEmail: "brand@example.com",
		Injectors: map[string]map[string]any{
			"brand": {
				"name":   "Request Brand",
				"locked": "REQUEST-SHOULD-NOT-WIN",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("expected outgoing email to be captured")
	}
	if captured.Subject != "Brand Request Brand LOCKED-GLOBAL" {
		t.Fatalf("unexpected subject: %q", captured.Subject)
	}
	if captured.BodyHTML != "<mjml><mj-body><mj-section><mj-column><mj-text>Name=Request Brand Locked=LOCKED-GLOBAL</mj-text></mj-column></mj-section></mj-body></mjml>" {
		t.Fatalf("unexpected body html: %q", captured.BodyHTML)
	}
}

func TestTestSendService_UsesTemplateTypeSenderIdentityWhenConfigured(t *testing.T) {
	tenantID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	templateTypeID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())
	senderIdentityID := uuid.Must(uuid.NewV7())

	templateStore := &mockTemplateStore{
		getPublishedVersionFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.TemplateVersion{
				ID:            uuid.Must(uuid.NewV7()),
				TemplateID:    templateID,
				VersionNumber: 1,
				Status:        domain.VersionStatusPublished,
				Subject:       "Shared SES test",
				BodyMJML:      "<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>",
				FromName:      "Support",
				DefaultLocale: "en",
			}, nil
		},
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: templateTypeID,
				WorkspaceID:    &workspaceID,
			}, nil
		},
		getTypeByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateType, error) {
			if id != templateTypeID {
				t.Fatalf("unexpected type id %s", id)
			}
			return &domain.TemplateType{
				ID:               templateTypeID,
				Slug:             "shared-ses-test",
				Name:             "Shared SES Test",
				AdapterID:        &adapterID,
				SenderIdentityID: &senderIdentityID,
			}, nil
		},
	}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.Adapter{
				ID:              adapterID,
				Name:            "SES Shared",
				AdapterType:     domain.AdapterTypeSES,
				ConfigEncrypted: []byte(`{}`),
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.AdapterIdentity, error) {
			if id != senderIdentityID {
				t.Fatalf("unexpected sender identity id %s", id)
			}
			return &domain.AdapterIdentity{
				ID:             senderIdentityID,
				AdapterID:      adapterID,
				Identity:       "shared@example.com",
				IdentityType:   domain.IdentityTypeEmail,
				Status:         domain.IdentityStatusVerified,
				SendingEnabled: true,
			}, nil
		},
		getDefaultFn: func(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
			return nil, domain.ErrNoDefaultIdentity
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id != workspaceID {
				t.Fatalf("unexpected workspace id %s", id)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: "main", Name: "Main"}, nil
		},
	}

	tenantStore := &mockTenantStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Tenant, error) {
			if id != tenantID {
				t.Fatalf("unexpected tenant id %s", id)
			}
			return &domain.Tenant{ID: tenantID, Code: "acme", Name: "Acme"}, nil
		},
	}

	var captured *port.OutgoingEmail
	svc := service.NewTestSendService(
		templateStore,
		adapterStore,
		identityStore,
		&mockCryptoTestSend{},
		&mockTemplateCompiler{compileFn: func(_ context.Context, mjml string) (string, error) { return mjml, nil }},
		service.NewVariableRenderer(),
		func(ctx context.Context, adapter *domain.Adapter, decrypted []byte) (port.EmailSender, error) {
			return &mockEmailSenderTestSend{
				sendFn: func(_ context.Context, msg *port.OutgoingEmail) (string, error) {
					captured = msg
					return "provider-msg-id", nil
				},
			}, nil
		},
		nil,
		tenantStore,
		wsStore,
	)

	result, err := svc.Send(context.Background(), &service.TestSendRequest{
		TemplateID:     templateID,
		RecipientEmail: "recipient@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FromAddress != "shared@example.com" {
		t.Fatalf("expected sender identity email, got %q", result.FromAddress)
	}
	if captured == nil {
		t.Fatal("expected outgoing email to be captured")
	}
	if captured.From.Address != "shared@example.com" {
		t.Fatalf("expected outgoing From address to use sender identity, got %q", captured.From.Address)
	}
}

func TestTestSendService_FallsBackToDelegateEmailWhenNoDefaultIdentity(t *testing.T) {
	tenantID := uuid.Must(uuid.NewV7())
	workspaceID := uuid.Must(uuid.NewV7())
	templateID := uuid.Must(uuid.NewV7())
	templateTypeID := uuid.Must(uuid.NewV7())
	adapterID := uuid.Must(uuid.NewV7())

	templateStore := &mockTemplateStore{
		getPublishedVersionFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateVersion, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.TemplateVersion{
				ID:            uuid.Must(uuid.NewV7()),
				TemplateID:    templateID,
				VersionNumber: 1,
				Status:        domain.VersionStatusPublished,
				Subject:       "Delegate fallback",
				BodyMJML:      "<mjml><mj-body><mj-section><mj-column><mj-text>Hello</mj-text></mj-column></mj-section></mj-body></mjml>",
				FromName:      "Support",
				DefaultLocale: "en",
			}, nil
		},
		getTemplateByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Template, error) {
			if id != templateID {
				t.Fatalf("unexpected template id %s", id)
			}
			return &domain.Template{
				ID:             templateID,
				TemplateTypeID: templateTypeID,
				WorkspaceID:    &workspaceID,
			}, nil
		},
		getTypeByIDFn: func(_ context.Context, id uuid.UUID) (*domain.TemplateType, error) {
			if id != templateTypeID {
				t.Fatalf("unexpected type id %s", id)
			}
			return &domain.TemplateType{
				ID:        templateTypeID,
				Slug:      "delegate-fallback",
				Name:      "Delegate fallback",
				AdapterID: &adapterID,
			}, nil
		},
	}

	adapterStore := &mockAdapterStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Adapter, error) {
			if id != adapterID {
				t.Fatalf("unexpected adapter id %s", id)
			}
			return &domain.Adapter{
				ID:              adapterID,
				Name:            "Gmail Shared",
				AdapterType:     domain.AdapterTypeGmail,
				ConfigEncrypted: []byte(`{"delegate_email":"delegate@example.com"}`),
				ConfigMeta: map[string]string{
					"delegate_email": "delegate@example.com",
				},
			}, nil
		},
	}

	identityStore := &mockAdapterIdentityStoreSend{
		getDefaultFn: func(_ context.Context, _ uuid.UUID) (*domain.AdapterIdentity, error) {
			return nil, domain.ErrNoDefaultIdentity
		},
	}

	wsStore := &mockWorkspaceStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Workspace, error) {
			if id != workspaceID {
				t.Fatalf("unexpected workspace id %s", id)
			}
			return &domain.Workspace{ID: workspaceID, TenantID: tenantID, Code: "main", Name: "Main"}, nil
		},
	}

	tenantStore := &mockTenantStoreSend{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Tenant, error) {
			if id != tenantID {
				t.Fatalf("unexpected tenant id %s", id)
			}
			return &domain.Tenant{ID: tenantID, Code: "acme", Name: "Acme"}, nil
		},
	}

	var captured *port.OutgoingEmail
	svc := service.NewTestSendService(
		templateStore,
		adapterStore,
		identityStore,
		&mockCryptoTestSend{},
		&mockTemplateCompiler{compileFn: func(_ context.Context, mjml string) (string, error) { return mjml, nil }},
		service.NewVariableRenderer(),
		func(ctx context.Context, adapter *domain.Adapter, decrypted []byte) (port.EmailSender, error) {
			return &mockEmailSenderTestSend{
				sendFn: func(_ context.Context, msg *port.OutgoingEmail) (string, error) {
					captured = msg
					return "provider-msg-id", nil
				},
			}, nil
		},
		nil,
		tenantStore,
		wsStore,
	)

	result, err := svc.Send(context.Background(), &service.TestSendRequest{
		TemplateID:     templateID,
		RecipientEmail: "recipient@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FromAddress != "delegate@example.com" {
		t.Fatalf("expected delegate_email fallback, got %q", result.FromAddress)
	}
	if captured == nil {
		t.Fatal("expected outgoing email to be captured")
	}
	if captured.From.Address != "delegate@example.com" {
		t.Fatalf("expected outgoing From address to use delegate_email fallback, got %q", captured.From.Address)
	}
}
