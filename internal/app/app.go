package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/senda-app/senda/config"
	"github.com/senda-app/senda/internal/adapter/crypto"
	"github.com/senda-app/senda/internal/adapter/mjml"
	"github.com/senda-app/senda/internal/adapter/pgcache"
	"github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/senda-app/senda/internal/adapter/river"
	smtpadapter "github.com/senda-app/senda/internal/adapter/smtp"
	"github.com/senda-app/senda/internal/adapter/sns"
	"github.com/senda-app/senda/internal/adapter/testauth"
	sendahttp "github.com/senda-app/senda/internal/http"
	"github.com/senda-app/senda/internal/http/handler"
	"github.com/senda-app/senda/internal/port"
	"github.com/senda-app/senda/internal/resolution"
	"github.com/senda-app/senda/internal/service"
)

// App holds the top-level application components for lifecycle management.
type App struct {
	Server      *sendahttp.Server
	RiverClient *river.Client
	Pool        *pgxpool.Pool
	cache       *pgcache.PGCache
}

// Bootstrap wires all dependencies and returns a ready-to-start App.
func Bootstrap(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*App, error) {
	// 1. Database connection.
	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("app: connect db: %w", err)
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
	domainRepo := postgres.NewDomainRepo(pool)
	webhookRepo := postgres.NewWebhookRepo(pool)
	suppressionRepo := postgres.NewSuppressionRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	configRepo := postgres.NewGlobalConfigRepo(pool)

	// 6. Resolution engine.
	chainResolver := resolution.NewChainResolver(wsRepo, cache)
	templateResolver := resolution.NewTemplateResolver(templateRepo, chainResolver)
	injectorMerger := resolution.NewInjectorMerger(injectorRepo, chainResolver)
	adapterResolver := resolution.NewAdapterResolver(adapterRepo, cache)
	domainResolver := resolution.NewDomainResolver(domainRepo, chainResolver, cache)

	// 7. Email sender (SMTP for dev/E2E, SES for production).
	var emailSender port.EmailSender
	if cfg.SMTP.Host != "" {
		emailSender = smtpadapter.NewAdapter(cfg.SMTP.Host, cfg.SMTP.Port)
		logger.Info("using SMTP email sender", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port)
	} else {
		logger.Warn("no email sender configured, send operations will fail")
	}

	// 8. River workers.
	sendWorker := river.NewSendWorker(emailRepo, domainRepo, aesCrypto, compiler, renderer, rateLimiter, emailSender)
	verifyWorker := river.NewVerifyWorker(domainRepo, nil)
	webhookWorker := river.NewWebhookWorker(webhookRepo, nil)

	riverClient, err := river.NewClient(pool, sendWorker, verifyWorker, webhookWorker)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: river client: %w", err)
	}

	// 9. Services.
	domainSvc := service.NewDomainService(domainRepo, aesCrypto, riverClient)
	webhookSvc := service.NewWebhookService(webhookRepo, riverClient)
	sendSvc := service.NewSendService(
		templateResolver, injectorMerger, adapterResolver, domainResolver,
		emailRepo, suppressionRepo, riverClient, renderer,
		tenantRepo, wsRepo,
	)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo)
	templateTypeSvc := service.NewTemplateTypeService(templateRepo)
	templateSvc := service.NewTemplateService(templateRepo, compiler)
	onboardingSvc := service.NewOnboardingService(memberRepo, tenantRepo, wsRepo, auditRepo)

	// 10. OIDC verifier.
	var oidcVerifier port.OIDCVerifier
	if cfg.OIDC.Mode == "test" {
		oidcVerifier = testauth.NewTestOIDCVerifier(cfg.OIDC.TestSecret)
		logger.Info("using test OIDC verifier (HS256 JWT)")
	} else {
		// Real OIDC verifier — not yet implemented, will fail at runtime.
		pool.Close()
		return nil, fmt.Errorf("app: real OIDC verifier not yet implemented; set SENDA_OIDC_MODE=test for E2E")
	}

	// 11. HTTP handlers.
	tenantH := handler.NewTenantHandler(tenantRepo, wsRepo)
	workspaceH := handler.NewWorkspaceHandler(tenantRepo, wsRepo)
	memberH := handler.NewMemberHandler(memberRepo)
	configH := handler.NewConfigHandler(configRepo)
	injectorH := handler.NewInjectorHandler(injectorRepo, tenantRepo, wsRepo)
	adapterH := handler.NewAdapterHandler(adapterRepo, aesCrypto, tenantRepo, wsRepo)
	domainH := handler.NewDomainHTTPHandler(domainSvc, domainRepo, tenantRepo, wsRepo)
	templateTypeH := handler.NewTemplateTypeHandler(templateTypeSvc, tenantRepo, wsRepo)
	templateH := handler.NewTemplateHandler(templateSvc, templateRepo, tenantRepo, wsRepo)
	sendH := handler.NewSendHandler(sendSvc)
	emailH := handler.NewEmailHandler(emailRepo, tenantRepo, wsRepo)
	suppressionH := handler.NewSuppressionHandler(suppressionRepo, tenantRepo, wsRepo)
	auditH := handler.NewAuditHandler(auditRepo, tenantRepo, wsRepo)
	webhookH := handler.NewWebhookHandler(webhookRepo, webhookSvc, tenantRepo, wsRepo)
	onboardingH := handler.NewOnboardingHandler(onboardingSvc, oidcVerifier)
	apiKeyH := handler.NewAPIKeyHandler(apiKeySvc, tenantRepo, wsRepo)

	// 12. SES webhook handler (only for SES mode, skip in SMTP/test mode).
	var sesOpts []sendahttp.ServerOption
	if cfg.SMTP.Host == "" {
		// Production mode: wire SES webhook handler with EventProcessor.
		eventProcessor := service.NewEventProcessor(emailRepo, emailRepo, suppressionRepo, webhookSvc, logger)
		snsVerifier := sns.NewVerifier(&http.Client{})
		snsConfirmer := handler.NewHTTPSubscriptionConfirmer(&http.Client{})
		sesH := handler.NewSESWebhookHandler(eventProcessor, snsVerifier, snsConfirmer, logger)
		sesOpts = append(sesOpts, sendahttp.WithSESWebhookHandler(sesH))
	}

	// 13. Assemble server.
	opts := []sendahttp.ServerOption{
		sendahttp.WithPinger(&dbPinger{pool: pool}),
		sendahttp.WithAuthDeps(apiKeyRepo, memberRepo, oidcVerifier),
		sendahttp.WithTenantStore(tenantRepo),
		sendahttp.WithWorkspaceStore(wsRepo),
		sendahttp.WithConfigStore(configRepo),
		sendahttp.WithTenantHandler(tenantH),
		sendahttp.WithWorkspaceHandler(workspaceH),
		sendahttp.WithMemberHandler(memberH),
		sendahttp.WithConfigHandler(configH),
		sendahttp.WithInjectorHandler(injectorH),
		sendahttp.WithAdapterHandler(adapterH),
		sendahttp.WithDomainHandler(domainH),
		sendahttp.WithDomainService(domainSvc),
		sendahttp.WithTemplateTypeHandler(templateTypeH),
		sendahttp.WithTemplateHandler(templateH),
		sendahttp.WithSendHandler(sendH),
		sendahttp.WithEmailHandler(emailH),
		sendahttp.WithSuppressionHandler(suppressionH),
		sendahttp.WithAuditHandler(auditH),
		sendahttp.WithWebhookHandler(webhookH),
		sendahttp.WithOnboardingHandler(onboardingH),
		sendahttp.WithAPIKeyHandler(apiKeyH),
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
