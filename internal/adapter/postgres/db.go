package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/config"
	"github.com/rendis/senda/pkg/apperr"
)

// Connect creates a new pgxpool.Pool from the given DatabaseConfig.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MinConns)
	poolCfg.MaxConnIdleTime = 5 * time.Minute

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
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

// classifyPgError maps known PostgreSQL constraint-violation error codes to
// typed AppErrors. Returns nil when the error is not a recognised PG error.
func classifyPgError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Code {
	case "23505": // unique_violation
		return apperr.Conflict("%s", pgErr.Detail)
	case "23503": // foreign_key_violation
		return apperr.BadRequest("referenced resource does not exist: %s", pgErr.Detail)
	case "23502": // not_null_violation
		return apperr.BadRequest("missing required field: %s", pgErr.ColumnName)
	default:
		return nil
	}
}

// coalesceJSON returns m if non-nil, otherwise an empty map. Prevents NULL on
// JSONB NOT NULL columns when the caller doesn't supply a value.
func coalesceJSON(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
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
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("reverting migrations: %w", err)
	}

	return nil
}
