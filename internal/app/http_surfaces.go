package app

import (
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
)

type managementSurfaceHandlers struct {
	tenant          *handler.TenantHandler
	workspace       *handler.WorkspaceHandler
	workspacePolicy *handler.WorkspacePolicyHandler
	member          *handler.MemberHandler
	config          *handler.ConfigHandler
	injector        *handler.InjectorHandler
	adapter         *handler.AdapterHandler
	identity        *handler.IdentityHandler
	adapterSetup    *handler.AdapterSetupHandler
	templateType    *handler.TemplateTypeHandler
	template        *handler.TemplateHandler
	email           *handler.EmailHandler
	suppression     *handler.SuppressionHandler
	audit           *handler.AuditHandler
	webhook         *handler.WebhookHandler
	apiKey          *handler.APIKeyHandler
	dashboard       *handler.DashboardHandler
}

func managementSurfaceOptions(h managementSurfaceHandlers) []sendahttp.ServerOption {
	return []sendahttp.ServerOption{
		sendahttp.WithTenantHandler(h.tenant),
		sendahttp.WithWorkspaceHandler(h.workspace),
		sendahttp.WithWorkspacePolicyHandler(h.workspacePolicy),
		sendahttp.WithMemberHandler(h.member),
		sendahttp.WithConfigHandler(h.config),
		sendahttp.WithInjectorHandler(h.injector),
		sendahttp.WithAdapterHandler(h.adapter),
		sendahttp.WithIdentityHandler(h.identity),
		sendahttp.WithTemplateTypeHandler(h.templateType),
		sendahttp.WithTemplateHandler(h.template),
		sendahttp.WithEmailHandler(h.email),
		sendahttp.WithSuppressionHandler(h.suppression),
		sendahttp.WithAuditHandler(h.audit),
		sendahttp.WithWebhookHandler(h.webhook),
		sendahttp.WithAPIKeyHandler(h.apiKey),
		sendahttp.WithAdapterSetupHandler(h.adapterSetup),
		sendahttp.WithDashboardHandler(h.dashboard),
	}
}

type dataPlaneSurfaceHandlers struct {
	send           *handler.SendHandler
	dataPlaneEmail *handler.DataPlaneEmailHandler
	sesWebhook     *handler.SESWebhookHandler
	onboarding     *handler.OnboardingHandler
}

func dataPlaneSurfaceOptions(h dataPlaneSurfaceHandlers) []sendahttp.ServerOption {
	return []sendahttp.ServerOption{
		sendahttp.WithSendHandler(h.send),
		sendahttp.WithDataPlaneEmailHandler(h.dataPlaneEmail),
		sendahttp.WithSESWebhookHandler(h.sesWebhook),
		sendahttp.WithOnboardingHandler(h.onboarding),
	}
}

type externalIntegrationSurfaceHandlers struct {
	externalIntegration *handler.ExternalIntegrationHandler
	injector            *handler.InjectorHandler
	workspacePolicy     *handler.WorkspacePolicyHandler
	templateType        *handler.TemplateTypeHandler
	template            *handler.TemplateHandler
}

func externalIntegrationSurfaceOptions(h externalIntegrationSurfaceHandlers) []sendahttp.ServerOption {
	return []sendahttp.ServerOption{
		sendahttp.WithExternalIntegrationHandler(h.externalIntegration),
		sendahttp.WithInjectorHandler(h.injector),
		sendahttp.WithWorkspacePolicyHandler(h.workspacePolicy),
		sendahttp.WithTemplateTypeHandler(h.templateType),
		sendahttp.WithTemplateHandler(h.template),
	}
}
