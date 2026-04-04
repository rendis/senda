package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/adapter/crypto"
	"github.com/rendis/senda/internal/adapter/mjml"
	"github.com/rendis/senda/internal/adapter/oidcauth"
	"github.com/rendis/senda/internal/adapter/pgcache"
	"github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/adapter/river"
	sesadapter "github.com/rendis/senda/internal/adapter/ses"
	smtpadapter "github.com/rendis/senda/internal/adapter/smtp"
	"github.com/rendis/senda/internal/adapter/sns"
	"github.com/rendis/senda/internal/adapter/testauth"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/http/handler"
	"github.com/rendis/senda/internal/port"
	"github.com/rendis/senda/internal/resolution"
	"github.com/rendis/senda/internal/service"
)

// App holds the top-level application components for lifecycle management.
type App struct {
	Server      *sendahttp.Server
	RiverClient *river.Client
	Pool        *pgxpool.Pool
	cache       *pgcache.PGCache
}

// Bootstrap wires all dependencies and returns a ready-to-start App.
// ext may be nil when running without SDK extensions.
func Bootstrap(ctx context.Context, cfg *config.Config, logger *slog.Logger, ext *Extensions) (*App, error) {
	// 1. Database connection.
	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("app: connect db: %w", err)
	}

	// 1b. Pre-heat connection pool to MinConns.
	for i := 0; i < cfg.Database.MinConns; i++ {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			break
		}
		conn.Release()
	}

	// 2. Run application migrations.
	if cfg.Database.MigrateOnStart {
		logger.Info("running database migrations")
		if err := postgres.RunMigrations(cfg.Database.URL, cfg.Database.MigrationsPath); err != nil {
			pool.Close()
			return nil, fmt.Errorf("app: run migrations: %w", err)
		}
	}

	// 3. Run River migrations.
	if err := runRiverMigrations(ctx, pool, logger); err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: river migrations: %w", err)
	}

	// 4. Infrastructure adapters.
	cache := pgcache.NewPGCache(pool)
	aesCrypto, err := crypto.NewAESCrypto(cfg.Crypto.MasterKey)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: init crypto: %w", err)
	}
	rateLimiter := postgres.NewProviderRateLimiter(pool)
	compiler := mjml.NewCompiler()
	renderer := service.NewVariableRenderer()

	// 5. Repository layer.
	tenantRepo := postgres.NewTenantRepo(pool)
	wsRepo := postgres.NewWorkspaceRepo(pool)
	memberRepo := postgres.NewMemberRepo(pool)
	apiKeyRepo := postgres.NewAPIKeyRepo(pool)
	emailRepo := postgres.NewEmailRepo(pool)
	templateRepo := postgres.NewTemplateRepo(pool)
	injectorRepo := postgres.NewInjectorRepo(pool)
	adapterRepo := postgres.NewAdapterRepo(pool)
	webhookRepo := postgres.NewWebhookRepo(pool)
	suppressionRepo := postgres.NewSuppressionRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	dashboardRepo := postgres.NewDashboardRepo(pool)
	configRepo := postgres.NewGlobalConfigRepo(pool)
	adapterIdentityRepo := postgres.NewAdapterIdentityRepo(pool)
	adapterGrantRepo := postgres.NewAdapterGrantRepo(pool)
	adapterIdentityGrantRepo := postgres.NewAdapterIdentityGrantRepo(pool)
	templateTypeUsageRepo := postgres.NewTemplateTypeUsageRepo(pool)

	// 6. Resolution engine.
	chainResolver := resolution.NewChainResolver(wsRepo, cache)
	templateResolver := resolution.NewTemplateResolver(templateRepo, cache, chainResolver)
	var codeInjectors []port.CodeInjector
	var codeInitFunc port.CodeInitFunc
	if ext != nil {
		codeInjectors = ext.Injectors
		codeInitFunc = ext.InitFunc
	}
	injectorMerger := resolution.NewInjectorMerger(injectorRepo, chainResolver, codeInjectors, codeInitFunc)
	adapterResolver := resolution.NewAdapterResolver(adapterRepo, cache)

	// 7. Email sender (SMTP for dev/E2E, SES for production).
	var emailSender port.EmailSender
	if cfg.SMTP.Host != "" {
		emailSender = smtpadapter.NewAdapter(cfg.SMTP.Host, cfg.SMTP.Port)
		logger.Info("using SMTP email sender", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port)
	} else {
		logger.Info("no static email sender configured; send worker will resolve adapter senders at runtime")
	}

	// 8. River workers.
	sendWorkerOpts := []river.SendWorkerOption{
		river.WithAdapterRuntime(adapterRepo, aesCrypto, river.DefaultAdapterSenderFactory),
	}
	if cfg.Tracking.BaseURL != "" {
		sendWorkerOpts = append(sendWorkerOpts,
			river.WithTrackingBaseURL(cfg.Tracking.BaseURL),
		)
		logger.Info("open tracking enabled", "base_url", cfg.Tracking.BaseURL)
	}
	sendWorker := river.NewSendWorker(emailRepo, compiler, renderer, rateLimiter, emailSender, sendWorkerOpts...)
	webhookWorker := river.NewWebhookWorker(webhookRepo, nil)

	riverClient, err := river.NewClient(pool, sendWorker, webhookWorker)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: river client: %w", err)
	}

	// 9. Services.
	webhookSvc := service.NewWebhookService(webhookRepo, riverClient)
	identitySvc := service.NewIdentityService(adapterIdentityRepo, adapterRepo, aesCrypto, DefaultIdentityProviderFactory)
	adapterAccessSvc := service.NewAdapterAccessService(
		adapterRepo,
		adapterIdentityRepo,
		wsRepo,
		adapterGrantRepo,
		adapterIdentityGrantRepo,
		templateTypeUsageRepo,
	)
	sendSvc := service.NewSendService(
		templateResolver, injectorMerger, adapterResolver,
		identitySvc,
		emailRepo, suppressionRepo, riverClient, renderer,
		tenantRepo, wsRepo,
		pool,
	)
	apiKeyPepper := deriveAPIKeyPepper(cfg.Crypto.MasterKey)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, apiKeyPepper)
	templateTypeSvc := service.NewTemplateTypeService(templateRepo)
	templateSvc := service.NewTemplateService(templateRepo, compiler)
	onboardingSvc := service.NewOnboardingService(pool, memberRepo, tenantRepo, wsRepo, auditRepo)

	// 10. OIDC verifier.
	var oidcVerifier port.OIDCVerifier
	switch cfg.OIDC.Mode {
	case "test":
		oidcVerifier = testauth.NewTestOIDCVerifier(cfg.OIDC.TestSecret)
		logger.Info("using test OIDC verifier (HS256 JWT)")
	case "dual":
		// Dual mode: try real OIDC first (Keycloak), fall back to test HS256.
		// Used in E2E so both the frontend (Keycloak tokens) and test suite (HS256) work.
		realVerifier, oidcErr := oidcauth.New(ctx, cfg.OIDC.DiscoveryURL, cfg.OIDC.ClientID, cfg.OIDC.SkipIssuerCheck)
		if oidcErr != nil {
			pool.Close()
			return nil, fmt.Errorf("app: OIDC verifier: %w", oidcErr)
		}
		testVerifier := testauth.NewTestOIDCVerifier(cfg.OIDC.TestSecret)
		oidcVerifier = testauth.NewChainVerifier(realVerifier, testVerifier)
		logger.Info("using dual OIDC verifier (real + test fallback)", "discovery_url", cfg.OIDC.DiscoveryURL)
	default:
		realVerifier, oidcErr := oidcauth.New(ctx, cfg.OIDC.DiscoveryURL, cfg.OIDC.ClientID, cfg.OIDC.SkipIssuerCheck)
		if oidcErr != nil {
			pool.Close()
			return nil, fmt.Errorf("app: OIDC verifier: %w", oidcErr)
		}
		oidcVerifier = realVerifier
		logger.Info("using real OIDC verifier", "discovery_url", cfg.OIDC.DiscoveryURL)
	}

	// 11. HTTP handlers.
	tenantH := handler.NewTenantHandler(tenantRepo, wsRepo, adapterRepo)
	workspaceH := handler.NewWorkspaceHandler(tenantRepo, wsRepo)
	memberH := handler.NewMemberHandler(memberRepo, tenantRepo, wsRepo)
	configH := handler.NewConfigHandler(configRepo, handler.OIDCInfo{
		DiscoveryURL:    cfg.OIDC.DiscoveryURL,
		ClientID:        cfg.OIDC.ClientID,
		ClientSecretSet: cfg.OIDC.ClientSecret != "",
	})
	injectorH := handler.NewInjectorHandler(injectorRepo, tenantRepo, wsRepo)
	// Tracking auto-provisioner (nil if no tracking base URL) — used for deprovision on adapter delete.
	provisioningStepRepo := postgres.NewProvisioningStepRepo(pool)
	var trackingProvisioner *sesadapter.TrackingProvisioner
	if cfg.Tracking.BaseURL != "" {
		trackingProvisioner = sesadapter.NewTrackingProvisioner(adapterRepo, aesCrypto, cfg.Tracking.BaseURL, logger, provisioningStepRepo)
	}
	adapterH := handler.NewAdapterHandler(adapterRepo, aesCrypto, tenantRepo, wsRepo,
		river.DefaultAdapterSenderFactory, adapterIdentityRepo, trackingProvisioner, logger)
	cacheInvalidator := resolution.NewCacheInvalidator(cache, wsRepo)
	templateTypeH := handler.NewTemplateTypeHandler(templateTypeSvc, tenantRepo, wsRepo, cacheInvalidator)
	testSendSvc := service.NewTestSendService(templateRepo, adapterRepo, adapterIdentityRepo, aesCrypto, compiler, renderer, river.DefaultAdapterSenderFactory)
	templateH := handler.NewTemplateHandler(templateSvc, templateRepo, tenantRepo, wsRepo, testSendSvc, sendSvc, auditRepo, cfg.Send.BatchMaxItems, cacheInvalidator)
	sendH := handler.NewSendHandler(sendSvc, cfg.Send.BatchMaxItems)
	emailH := handler.NewEmailHandler(emailRepo, tenantRepo, wsRepo)
	dataPlaneEmailH := handler.NewDataPlaneEmailHandler(emailRepo)
	suppressionH := handler.NewSuppressionHandler(suppressionRepo, tenantRepo, wsRepo)
	auditH := handler.NewAuditHandler(auditRepo, tenantRepo, wsRepo)
	webhookH := handler.NewWebhookHandler(webhookRepo, webhookSvc, tenantRepo, wsRepo)
	onboardingH := handler.NewOnboardingHandler(onboardingSvc, oidcVerifier)
	identityH := handler.NewIdentityHandler(identitySvc, adapterIdentityRepo, tenantRepo, wsRepo)
	adapterSetupH := handler.NewAdapterSetupHandler(adapterRepo, tenantRepo, wsRepo, cfg.Tracking.BaseURL, trackingProvisioner, provisioningStepRepo)
	apiKeyH := handler.NewAPIKeyHandler(apiKeySvc, tenantRepo, wsRepo)
	dashboardH := handler.NewDashboardHandler(dashboardRepo, auditRepo, tenantRepo, wsRepo)
	adapterH.SetAdapterAccessService(adapterAccessSvc)
	adapterH.SetAuditStore(auditRepo)
	templateTypeH.SetAdapterAccessService(adapterAccessSvc)
	identityH.SetAdapterAccessService(adapterAccessSvc)
	identityH.SetAuditStore(auditRepo)
	sendSvc.SetAdapterAccessService(adapterAccessSvc)

	// 12. Event processor (shared by SES webhook + open-tracking pixel).
	eventProcessor := service.NewEventProcessor(emailRepo, emailRepo, suppressionRepo, webhookSvc, logger)

	// 13. SES webhook handler (only for SES mode, skip in SMTP/test mode).
	var sesOpts []sendahttp.ServerOption
	if cfg.SMTP.Host == "" {
		snsVerifier := sns.NewVerifier(&http.Client{})
		snsConfirmer := handler.NewHTTPSubscriptionConfirmer(&http.Client{})
		sesH := handler.NewSESWebhookHandler(
			eventProcessor,
			snsVerifier,
			snsConfirmer,
			logger,
			handler.WithSkipSignatureVerification(cfg.SNS.SkipSignatureVerification),
		)
		sesOpts = append(sesOpts, sendahttp.WithSESWebhookHandler(sesH))
	}

	// 14. Open-tracking handler.
	trackingH := handler.NewTrackingHandler(ctx, emailRepo, eventProcessor, logger)

	// 14b. Media handler (video thumbnail composite).
	mediaH := handler.NewMediaHandler(logger)

	// 15. Assemble server.
	opts := []sendahttp.ServerOption{
		sendahttp.WithPinger(&dbPinger{pool: pool}),
		sendahttp.WithAuthDeps(apiKeyRepo, memberRepo, oidcVerifier, apiKeyPepper),
		sendahttp.WithTenantStore(tenantRepo),
		sendahttp.WithWorkspaceStore(wsRepo),
		sendahttp.WithConfigStore(configRepo),
		sendahttp.WithTenantHandler(tenantH),
		sendahttp.WithWorkspaceHandler(workspaceH),
		sendahttp.WithMemberHandler(memberH),
		sendahttp.WithConfigHandler(configH),
		sendahttp.WithInjectorHandler(injectorH),
		sendahttp.WithAdapterHandler(adapterH),
		sendahttp.WithIdentityHandler(identityH),
		sendahttp.WithTemplateTypeHandler(templateTypeH),
		sendahttp.WithTemplateHandler(templateH),
		sendahttp.WithSendHandler(sendH),
		sendahttp.WithDataPlaneEmailHandler(dataPlaneEmailH),
		sendahttp.WithEmailHandler(emailH),
		sendahttp.WithSuppressionHandler(suppressionH),
		sendahttp.WithAuditHandler(auditH),
		sendahttp.WithWebhookHandler(webhookH),
		sendahttp.WithOnboardingHandler(onboardingH),
		sendahttp.WithAPIKeyHandler(apiKeyH),
		sendahttp.WithAdapterSetupHandler(adapterSetupH),
		sendahttp.WithTrackingHandler(trackingH),
		sendahttp.WithMediaHandler(mediaH),
		sendahttp.WithDashboardHandler(dashboardH),
	}
	opts = append(opts, sesOpts...)

	srv := sendahttp.NewServer(cfg, logger, opts...)

	return &App{
		Server:      srv,
		RiverClient: riverClient,
		Pool:        pool,
		cache:       cache,
	}, nil
}

// Close gracefully shuts down app resources.
func (a *App) Close(ctx context.Context) {
	if a.RiverClient != nil {
		if err := a.RiverClient.Stop(ctx); err != nil {
			slog.Error("stopping river client", "error", err)
		}
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
}

// dbPinger adapts pgxpool.Pool to the handler.Pinger interface.
type dbPinger struct {
	pool *pgxpool.Pool
}

func (p *dbPinger) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// runRiverMigrations applies River's internal schema migrations.
func runRiverMigrations(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}

	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("run river migrations: %w", err)
	}

	if len(res.Versions) > 0 {
		logger.Info("river migrations applied", "versions", len(res.Versions))
	}
	return nil
}

// deriveAPIKeyPepper derives a pepper for API key HMAC from the master key using HMAC-SHA256.
func deriveAPIKeyPepper(masterKey string) string {
	mac := hmac.New(sha256.New, []byte(masterKey))
	mac.Write([]byte("senda-api-key-pepper-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}
