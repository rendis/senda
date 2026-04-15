package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/rendis/senda/config"
	"github.com/rendis/senda/internal/adapter/pgcache"
	"github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/adapter/river"
	sendahttp "github.com/rendis/senda/internal/http"
	"github.com/rendis/senda/internal/port"
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

	infra, err := newInfraBundle(cfg, logger, pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: %w", err)
	}

	repos := newRepositoryBundle(pool)
	resolvers := newResolutionBundle(repos, infra.cache, ext, logger)

	riverClient, err := newRiverClient(cfg, logger, pool, repos, infra)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: river client: %w", err)
	}

	services := newServiceBundle(cfg, pool, repos, infra, resolvers, riverClient, logger)

	oidcVerifier, err := newOIDCVerifier(ctx, cfg, logger)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: %w", err)
	}

	handlers, err := newServerHandlerBundle(ctx, cfg, logger, ext, repos, infra, resolvers, services, oidcVerifier)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: handlers: %w", err)
	}
	apiKeyPepper := deriveAPIKeyPepper(cfg.Crypto.MasterKey)

	// 15. Assemble server.
	opts := newServerOptions(
		serverSharedDeps{
			pinger:         &dbPinger{pool: pool},
			apiKeyStore:    repos.apiKeyRepo,
			memberStore:    repos.memberRepo,
			oidcVerifier:   oidcVerifier,
			apiKeyPepper:   apiKeyPepper,
			tenantStore:    repos.tenantRepo,
			workspaceStore: repos.workspaceRepo,
			configStore:    repos.configRepo,
		},
		handlers,
	)

	srv := sendahttp.NewServer(cfg, logger, opts...)

	return &App{
		Server:      srv,
		RiverClient: riverClient,
		Pool:        pool,
		cache:       infra.cache,
	}, nil
}

func extExternalAuthMethods(ext *Extensions) []port.ExternalAuthMethod {
	if ext == nil {
		return nil
	}
	return ext.ExternalAuthMethods
}

func extExternalResolvers(ext *Extensions) []port.ExternalWorkspaceResolver {
	if ext == nil {
		return nil
	}
	return ext.ExternalWorkspaceResolvers
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
