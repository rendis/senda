package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/senda-app/senda/config"
)

// Connect creates a new pgxpool.Pool from the given DatabaseConfig.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)

	if cfg.ConnMaxLifetime != "" {
		d, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			return nil, fmt.Errorf("parsing conn_max_lifetime %q: %w", cfg.ConnMaxLifetime, err)
		}
		poolCfg.MaxConnLifetime = d
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// HealthCheck verifies that the database connection is alive.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// toPgx5URL converts a postgres:// or postgresql:// URL to a pgx5:// URL
// that golang-migrate's pgx/v5 driver expects.
func toPgx5URL(dbURL string) string {
	u := strings.Replace(dbURL, "postgresql://", "pgx5://", 1)
	u = strings.Replace(u, "postgres://", "pgx5://", 1)
	return u
}

// RunMigrations applies all pending migrations from the given path.
func RunMigrations(dbURL string, migrationsPath string) error {
	m, err := migrate.New(
		"file://"+migrationsPath,
		toPgx5URL(dbURL),
	)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

// RunMigrationsDown reverts all migrations from the given path.
func RunMigrationsDown(dbURL string, migrationsPath string) error {
	m, err := migrate.New(
		"file://"+migrationsPath,
		toPgx5URL(dbURL),
	)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("reverting migrations: %w", err)
	}

	return nil
}
