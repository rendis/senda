package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rendis/senda/config"
	cryptoadapter "github.com/rendis/senda/internal/adapter/crypto"
	"github.com/rendis/senda/internal/adapter/mjml"
	"github.com/rendis/senda/internal/adapter/oidcauth"
	"github.com/rendis/senda/internal/adapter/pgcache"
	"github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/adapter/river"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
	"github.com/rendis/senda/internal/adapter/testauth"
	"github.com/rendis/senda/internal/domain"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/service"
)

type serverSharedDeps struct {
	pinger         handler.Pinger
	apiKeyStore    port.APIKeyStore
	memberStore    port.MemberStore
	oidcVerifier   port.OIDCVerifier
	apiKeyPepper   string
	tenantStore    port.TenantStore
	workspaceStore port.WorkspaceStore
	configStore    port.GlobalConfigStore
}

type serverHandlerBundle struct {
	tenantHandler              *handler.TenantHandler
	workspaceHandler           *handler.WorkspaceHandler
	workspacePolicyHandler     *handler.WorkspacePolicyHandler
	memberHandler              *handler.MemberHandler
	configHandler              *handler.ConfigHandler
	externalIntegrationHandler *handler.ExternalIntegrationHandler
	injectorHandler            *handler.InjectorHandler
	adapterHandler             *handler.AdapterHandler
	identityHandler            *handler.IdentityHandler
	templateTypeHandler        *handler.TemplateTypeHandler
	templateHandler            *handler.TemplateHandler
	sendHandler                *handler.SendHandler
	dataPlaneEmailHandler      *handler.DataPlaneEmailHandler
	emailHandler               *handler.EmailHandler
	suppressionHandler         *handler.SuppressionHandler
	auditHandler               *handler.AuditHandler
	webhookHandler             *handler.WebhookHandler
	onboardingHandler          *handler.OnboardingHandler
	apiKeyHandler              *handler.APIKeyHandler
	adapterSetupHandler        *handler.AdapterSetupHandler
	sesWebhookHandler          *handler.SESWebhookHandler
	trackingHandler            *handler.TrackingHandler
	mediaHandler               *handler.MediaHandler
	dashboardHandler           *handler.DashboardHandler
}

type infraBundle struct {
	cache       *pgcache.PGCache
	aesCrypto   port.Crypto
	rateLimiter port.RateLimiter
	compiler    port.TemplateCompiler
	renderer    port.VariableRenderer
	emailSender port.EmailSender
}

type repositoryBundle struct {
	tenantRepo               port.TenantStore
	workspaceRepo            port.WorkspaceStore
	memberRepo               port.MemberStore
	apiKeyRepo               port.APIKeyStore
	emailRepo                port.EmailStore
	templateRepo             port.TemplateStore
	injectorRepo             port.InjectorStore
	adapterRepo              port.AdapterStore
	webhookRepo              port.WebhookStore
	suppressionRepo          port.SuppressionStore
	auditRepo                port.AuditLogStore
	dashboardRepo            port.DashboardStore
	configRepo               port.GlobalConfigStore
	adapterIdentityRepo      port.AdapterIdentityStore
	adapterGrantRepo         port.AdapterGrantStore
	adapterIdentityGrantRepo port.AdapterIdentityGrantStore
	templateTypeUsageRepo    port.TemplateTypeUsageStore
	provisioningStepRepo     port.ProvisioningStepStore
	snsReplayRepo            port.SNSReplayStore
}

type resolutionBundle struct {
	templateResolver *resolution.TemplateResolver
	injectorMerger   *resolution.InjectorMerger
	adapterResolver  *resolution.AdapterResolver
	cacheInvalidator *resolution.CacheInvalidator
}

type serviceBundle struct {
	webhookSvc       *service.WebhookService
	identitySvc      *service.IdentityService
	adapterAccessSvc *service.AdapterAccessService
	sendSvc          *service.SendService
	apiKeySvc        *service.APIKeyService
	templateTypeSvc  *service.TemplateTypeService
	templateSvc      *service.TemplateService
	onboardingSvc    *service.OnboardingService
	testSendSvc      *service.TestSendService
	eventProcessor   *service.EventProcessor
}

func newServerOptions(shared serverSharedDeps, handlers serverHandlerBundle) []sendahttp.ServerOption {
	return []sendahttp.ServerOption{
		sendahttp.WithPinger(shared.pinger),
		sendahttp.WithAuthDeps(shared.apiKeyStore, shared.memberStore, shared.oidcVerifier, shared.apiKeyPepper),
		sendahttp.WithTenantStore(shared.tenantStore),
		sendahttp.WithWorkspaceStore(shared.workspaceStore),
		sendahttp.WithConfigStore(shared.configStore),
		sendahttp.WithTenantHandler(handlers.tenantHandler),
		sendahttp.WithWorkspaceHandler(handlers.workspaceHandler),
		sendahttp.WithWorkspacePolicyHandler(handlers.workspacePolicyHandler),
		sendahttp.WithMemberHandler(handlers.memberHandler),
		sendahttp.WithConfigHandler(handlers.configHandler),
		sendahttp.WithExternalIntegrationHandler(handlers.externalIntegrationHandler),
		sendahttp.WithInjectorHandler(handlers.injectorHandler),
		sendahttp.WithAdapterHandler(handlers.adapterHandler),
		sendahttp.WithIdentityHandler(handlers.identityHandler),
		sendahttp.WithTemplateTypeHandler(handlers.templateTypeHandler),
		sendahttp.WithTemplateHandler(handlers.templateHandler),
		sendahttp.WithSendHandler(handlers.sendHandler),
		sendahttp.WithDataPlaneEmailHandler(handlers.dataPlaneEmailHandler),
		sendahttp.WithEmailHandler(handlers.emailHandler),
		sendahttp.WithSuppressionHandler(handlers.suppressionHandler),
		sendahttp.WithAuditHandler(handlers.auditHandler),
		sendahttp.WithWebhookHandler(handlers.webhookHandler),
		sendahttp.WithOnboardingHandler(handlers.onboardingHandler),
		sendahttp.WithAPIKeyHandler(handlers.apiKeyHandler),
		sendahttp.WithAdapterSetupHandler(handlers.adapterSetupHandler),
		sendahttp.WithTrackingHandler(handlers.trackingHandler),
		sendahttp.WithMediaHandler(handlers.mediaHandler),
		sendahttp.WithDashboardHandler(handlers.dashboardHandler),
		sendahttp.WithSESWebhookHandler(handlers.sesWebhookHandler),
	}
}

func newInfraBundle(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool) (*infraBundle, error) {
	cache := pgcache.NewPGCache(pool)

	aesCrypto, err := cryptoadapter.NewAESCrypto(cfg.Crypto.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("init crypto: %w", err)
	}

	var emailSender port.EmailSender
	if cfg.SMTP.Host != "" {
		emailSender = smtpadapter.NewAdapter(cfg.SMTP.Host, cfg.SMTP.Port)
		logger.Info("using SMTP email sender", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port)
	} else {
		logger.Info("no static email sender configured; send worker will resolve adapter senders at runtime")
	}

	return &infraBundle{
		cache:       cache,
		aesCrypto:   aesCrypto,
		rateLimiter: postgres.NewProviderRateLimiter(pool),
		compiler:    mjml.NewCompiler(mjml.WithPublicBaseURL(cfg.Tracking.BaseURL)),
		renderer:    service.NewVariableRenderer(),
		emailSender: emailSender,
	}, nil
}

func newRepositoryBundle(pool *pgxpool.Pool) repositoryBundle {
	return repositoryBundle{
		tenantRepo:               postgres.NewTenantRepo(pool),
		workspaceRepo:            postgres.NewWorkspaceRepo(pool),
		memberRepo:               postgres.NewMemberRepo(pool),
		apiKeyRepo:               postgres.NewAPIKeyRepo(pool),
		emailRepo:                postgres.NewEmailRepo(pool),
		templateRepo:             postgres.NewTemplateRepo(pool),
		injectorRepo:             postgres.NewInjectorRepo(pool),
		adapterRepo:              postgres.NewAdapterRepo(pool),
		webhookRepo:              postgres.NewWebhookRepo(pool),
		suppressionRepo:          postgres.NewSuppressionRepo(pool),
		auditRepo:                postgres.NewAuditRepo(pool),
		dashboardRepo:            postgres.NewDashboardRepo(pool),
		configRepo:               postgres.NewGlobalConfigRepo(pool),
		adapterIdentityRepo:      postgres.NewAdapterIdentityRepo(pool),
		adapterGrantRepo:         postgres.NewAdapterGrantRepo(pool),
		adapterIdentityGrantRepo: postgres.NewAdapterIdentityGrantRepo(pool),
		templateTypeUsageRepo:    postgres.NewTemplateTypeUsageRepo(pool),
		provisioningStepRepo:     postgres.NewProvisioningStepRepo(pool),
		snsReplayRepo:            postgres.NewSNSReplayRepo(pool),
	}
}

func newResolutionBundle(repos repositoryBundle, cache *pgcache.PGCache, ext *Extensions, logger *slog.Logger) resolutionBundle {
	var codeInjectors []port.CodeInjector
	var codeInitFunc port.CodeInitFunc
	if ext != nil {
		codeInjectors = ext.Injectors
		codeInitFunc = ext.InitFunc
	}
	if len(codeInjectors) > 0 || codeInitFunc != nil {
		logger.Info("registered runtime code injector extensions", "injector_count", len(codeInjectors), "has_init_func", codeInitFunc != nil)
	}

	chainResolver := resolution.NewChainResolver(repos.workspaceRepo, cache)
	return resolutionBundle{
		templateResolver: resolution.NewTemplateResolver(repos.templateRepo, cache, chainResolver),
		injectorMerger:   resolution.NewInjectorMerger(repos.injectorRepo, chainResolver, cache, codeInjectors, codeInitFunc),
		adapterResolver:  resolution.NewAdapterResolver(repos.adapterRepo, cache),
		cacheInvalidator: resolution.NewCacheInvalidator(cache, repos.workspaceRepo),
	}
}

func newRiverClient(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool, repos repositoryBundle, infra *infraBundle) (*river.Client, error) {
	sendWorkerOpts := []river.SendWorkerOption{
		river.WithAdapterRuntime(repos.adapterRepo, infra.aesCrypto, river.DefaultAdapterSenderFactory),
	}
	if cfg.Tracking.BaseURL != "" {
		sendWorkerOpts = append(sendWorkerOpts, river.WithTrackingBaseURL(cfg.Tracking.BaseURL))
		logger.Info("open tracking enabled", "base_url", cfg.Tracking.BaseURL)
	}

	sendWorker := river.NewSendWorker(repos.emailRepo, infra.compiler, infra.renderer, infra.rateLimiter, infra.emailSender, sendWorkerOpts...)
	webhookWorker := river.NewWebhookWorker(repos.webhookRepo, nil)
	return river.NewClient(pool, sendWorker, webhookWorker)
}

func newOIDCVerifier(ctx context.Context, cfg *config.Config, logger *slog.Logger) (port.OIDCVerifier, error) {
	switch cfg.OIDC.Mode {
	case "test":
		logger.Info("using test OIDC verifier (HS256 JWT)")
		return testauth.NewTestOIDCVerifier(cfg.OIDC.TestSecret), nil
	case "dual":
		realVerifier, err := oidcauth.New(ctx, cfg.OIDC.DiscoveryURL, cfg.OIDC.ClientID, cfg.OIDC.SkipIssuerCheck)
		if err != nil {
			return nil, fmt.Errorf("OIDC verifier: %w", err)
		}
		logger.Info("using dual OIDC verifier (real + test fallback)", "discovery_url", cfg.OIDC.DiscoveryURL)
		return testauth.NewChainVerifier(realVerifier, testauth.NewTestOIDCVerifier(cfg.OIDC.TestSecret)), nil
	default:
		realVerifier, err := oidcauth.New(ctx, cfg.OIDC.DiscoveryURL, cfg.OIDC.ClientID, cfg.OIDC.SkipIssuerCheck)
		if err != nil {
			return nil, fmt.Errorf("OIDC verifier: %w", err)
		}
		logger.Info("using real OIDC verifier", "discovery_url", cfg.OIDC.DiscoveryURL)
		return realVerifier, nil
	}
}

func newServiceBundle(cfg *config.Config, pool *pgxpool.Pool, repos repositoryBundle, infra *infraBundle, resolvers resolutionBundle, riverClient *river.Client, logger *slog.Logger) serviceBundle {
	webhookSvc := service.NewWebhookService(repos.webhookRepo, riverClient)
	identitySvc := service.NewIdentityService(repos.adapterIdentityRepo, repos.adapterRepo, infra.aesCrypto, DefaultIdentityProviderFactory)
	adapterAccessSvc := service.NewAdapterAccessService(
		repos.adapterRepo,
		repos.adapterIdentityRepo,
		repos.workspaceRepo,
		repos.adapterGrantRepo,
		repos.adapterIdentityGrantRepo,
		repos.templateTypeUsageRepo,
	)
	sendSvc := service.NewSendService(
		resolvers.templateResolver,
		resolvers.injectorMerger,
		resolvers.adapterResolver,
		identitySvc,
		repos.emailRepo,
		repos.suppressionRepo,
		riverClient,
		infra.renderer,
		repos.tenantRepo,
		repos.workspaceRepo,
		pool,
	)
	apiKeySvc := service.NewAPIKeyService(repos.apiKeyRepo, deriveAPIKeyPepper(cfg.Crypto.MasterKey))
	templateTypeSvc := service.NewTemplateTypeService(repos.templateRepo)
	templateSvc := service.NewTemplateService(repos.templateRepo, infra.compiler)
	onboardingSvc := service.NewOnboardingService(pool, repos.memberRepo, repos.tenantRepo, repos.workspaceRepo, repos.auditRepo)

	testSendSenderFactory := river.DefaultAdapterSenderFactory
	if infra.emailSender != nil {
		testSendSenderFactory = func(context.Context, *domain.Adapter, []byte) (port.EmailSender, error) {
			return infra.emailSender, nil
		}
	}

	return serviceBundle{
		webhookSvc:       webhookSvc,
		identitySvc:      identitySvc,
		adapterAccessSvc: adapterAccessSvc,
		sendSvc:          sendSvc,
		apiKeySvc:        apiKeySvc,
		templateTypeSvc:  templateTypeSvc,
		templateSvc:      templateSvc,
		onboardingSvc:    onboardingSvc,
		testSendSvc: service.NewTestSendService(
			repos.templateRepo,
			repos.adapterRepo,
			repos.adapterIdentityRepo,
			infra.aesCrypto,
			infra.compiler,
			infra.renderer,
			testSendSenderFactory,
			resolvers.injectorMerger,
			repos.tenantRepo,
			repos.workspaceRepo,
		),
		eventProcessor: service.NewEventProcessor(repos.emailRepo, repos.emailRepo, repos.suppressionRepo, webhookSvc, logger),
	}
}

func newServerHandlerBundle(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	ext *Extensions,
	repos repositoryBundle,
	infra *infraBundle,
	resolvers resolutionBundle,
	services serviceBundle,
	oidcVerifier port.OIDCVerifier,
) (serverHandlerBundle, error) {
	tenantH := handler.NewTenantHandler(repos.tenantRepo, repos.workspaceRepo, repos.adapterRepo)
	workspaceH := handler.NewWorkspaceHandler(repos.tenantRepo, repos.workspaceRepo, repos.emailRepo)
	workspacePolicyH := handler.NewWorkspacePolicyHandler(repos.tenantRepo, repos.workspaceRepo)
	memberH := handler.NewMemberHandler(repos.memberRepo, repos.tenantRepo, repos.workspaceRepo)
	configH := handler.NewConfigHandler(repos.configRepo, handler.OIDCInfo{
		DiscoveryURL:    cfg.OIDC.DiscoveryURL,
		ClientID:        cfg.OIDC.ClientID,
		ClientSecretSet: cfg.OIDC.ClientSecret != "",
	})
	externalIntegrationH := handler.NewExternalIntegrationHandler(repos.configRepo, extExternalAuthMethods(ext), extExternalResolvers(ext))
	injectorH := handler.NewInjectorHandler(repos.injectorRepo, repos.tenantRepo, repos.workspaceRepo, resolvers.injectorMerger)

	var trackingProvisioner *sesadapter.TrackingProvisioner
	if cfg.Tracking.BaseURL != "" {
		trackingProvisioner = sesadapter.NewTrackingProvisioner(repos.adapterRepo, infra.aesCrypto, cfg.Tracking.BaseURL, logger, repos.provisioningStepRepo)
	}

	adapterH := handler.NewAdapterHandler(repos.adapterRepo, infra.aesCrypto, repos.tenantRepo, repos.workspaceRepo, river.DefaultAdapterSenderFactory, repos.adapterIdentityRepo, trackingProvisioner, logger)
	templateTypeH := handler.NewTemplateTypeHandler(services.templateTypeSvc, repos.tenantRepo, repos.workspaceRepo, resolvers.cacheInvalidator)
	templateH := handler.NewTemplateHandler(services.templateSvc, repos.templateRepo, repos.tenantRepo, repos.workspaceRepo, services.testSendSvc, services.sendSvc, repos.auditRepo, cfg.Send.BatchMaxItems, resolvers.injectorMerger, resolvers.cacheInvalidator)
	sendH := handler.NewSendHandler(services.sendSvc, cfg.Send.BatchMaxItems)
	emailH := handler.NewEmailHandler(repos.emailRepo, repos.tenantRepo, repos.workspaceRepo)
	dataPlaneEmailH := handler.NewDataPlaneEmailHandler(repos.emailRepo)
	suppressionH := handler.NewSuppressionHandler(repos.suppressionRepo, repos.tenantRepo, repos.workspaceRepo)
	auditH := handler.NewAuditHandler(repos.auditRepo, repos.tenantRepo, repos.workspaceRepo)
	webhookH := handler.NewWebhookHandler(repos.webhookRepo, services.webhookSvc, repos.tenantRepo, repos.workspaceRepo)
	onboardingH := handler.NewOnboardingHandler(services.onboardingSvc, oidcVerifier)
	identityH := handler.NewIdentityHandler(services.identitySvc, repos.adapterIdentityRepo, repos.tenantRepo, repos.workspaceRepo)
	adapterSetupH := handler.NewAdapterSetupHandler(repos.adapterRepo, repos.tenantRepo, repos.workspaceRepo, cfg.Tracking.BaseURL, trackingProvisioner, repos.provisioningStepRepo)
	apiKeyH := handler.NewAPIKeyHandler(services.apiKeySvc, repos.tenantRepo, repos.workspaceRepo)
	dashboardH := handler.NewDashboardHandler(repos.dashboardRepo, repos.auditRepo, repos.tenantRepo, repos.workspaceRepo)

	adapterH.SetAdapterAccessService(services.adapterAccessSvc)
	adapterH.SetAuditStore(repos.auditRepo)
	templateTypeH.SetAdapterAccessService(services.adapterAccessSvc)
	identityH.SetAdapterAccessService(services.adapterAccessSvc)
	identityH.SetAuditStore(repos.auditRepo)
	services.sendSvc.SetAdapterAccessService(services.adapterAccessSvc)

	var sesWebhookHandler *handler.SESWebhookHandler
	if cfg.SMTP.Host == "" {
		var err error
		sesWebhookHandler, err = buildSESWebhookHandler(cfg, services.eventProcessor, logger, repos.snsReplayRepo)
		if err != nil {
			return serverHandlerBundle{}, err
		}
	}
	if trackingProvisioner != nil && sesWebhookHandler != nil {
		trackingProvisioner.SetSNSBindingRegistrar(sesWebhookHandler)
	}

	return serverHandlerBundle{
		tenantHandler:              tenantH,
		workspaceHandler:           workspaceH,
		workspacePolicyHandler:     workspacePolicyH,
		memberHandler:              memberH,
		configHandler:              configH,
		externalIntegrationHandler: externalIntegrationH,
		injectorHandler:            injectorH,
		adapterHandler:             adapterH,
		identityHandler:            identityH,
		templateTypeHandler:        templateTypeH,
		templateHandler:            templateH,
		sendHandler:                sendH,
		dataPlaneEmailHandler:      dataPlaneEmailH,
		emailHandler:               emailH,
		suppressionHandler:         suppressionH,
		auditHandler:               auditH,
		webhookHandler:             webhookH,
		onboardingHandler:          onboardingH,
		apiKeyHandler:              apiKeyH,
		adapterSetupHandler:        adapterSetupH,
		sesWebhookHandler:          sesWebhookHandler,
		trackingHandler:            handler.NewTrackingHandler(ctx, repos.emailRepo, services.eventProcessor, logger),
		mediaHandler:               buildMediaHandler(cfg, logger),
		dashboardHandler:           dashboardH,
	}, nil
}
