//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/config"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
)

func TestConnect(t *testing.T) {
	ctx := context.Background()
	connStr := sharedConnStr(ctx, t)

	cfg := config.DatabaseConfig{
		URL:             connStr,
		MaxOpenConns:    5,
		MinConns:    2,
		ConnMaxLifetime: "5m",
		MigrateOnStart:  false,
	}

	pool, err := pgadapter.Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer pool.Close()

	if err := pgadapter.HealthCheck(ctx, pool); err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
}

func TestMigrations(t *testing.T) {
	ctx := context.Background()
	sharedDBMu.Lock()
	t.Cleanup(func() {
		sharedDBMu.Unlock()
	})
	connStr := sharedConnStr(ctx, t)

	// Ensure deterministic baseline for this migration test.
	if err := pgadapter.RunMigrationsDown(connStr, migrationsPath()); err != nil {
		t.Fatalf("pre-reset RunMigrationsDown() error: %v", err)
	}

	// Run all migrations up
	if err := pgadapter.RunMigrations(connStr, migrationsPath()); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	// Connect to verify tables exist
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("connecting to verify: %v", err)
	}
	defer pool.Close()

	// Verify key tables exist
	expectedTables := []string{
		"tenants",
		"workspaces",
		"injector_definitions",
		"injector_fields",
		"injector_values",
		"adapters",
		"template_types",
		"templates",
		"members",
		"member_roles",
		"template_versions",
		"template_version_locales",
		"api_keys",
		"emails",
		"email_events",
		"suppression_global",
		"suppression_workspace",
		"webhooks",
		"audit_logs",
		"global_config",
		"cache",
		"token_buckets",
	}

	for _, table := range expectedTables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Errorf("checking table %s: %v", table, err)
			continue
		}
		if !exists {
			t.Errorf("expected table %s to exist after migration up", table)
		}
	}

	// Verify key functions exist
	expectedFunctions := []string{
		"get_resolution_chain",
		"take_send_token",
	}

	for _, fn := range expectedFunctions {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.routines WHERE routine_schema = 'public' AND routine_name = $1)",
			fn,
		).Scan(&exists)
		if err != nil {
			t.Errorf("checking function %s: %v", fn, err)
			continue
		}
		if !exists {
			t.Errorf("expected function %s to exist after migration up", fn)
		}
	}

	// Verify global_config seed data
	var configCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM global_config").Scan(&configCount)
	if err != nil {
		t.Fatalf("counting global_config: %v", err)
	}
	if configCount != 10 {
		t.Errorf("expected 10 global_config rows, got %d", configCount)
	}

	// Verify pg_cron jobs exist
	var cronCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM cron.job WHERE jobname IN ('cache-cleanup', 'create-partitions')").Scan(&cronCount)
	if err != nil {
		t.Fatalf("counting cron jobs: %v", err)
	}
	if cronCount != 2 {
		t.Errorf("expected 2 cron jobs, got %d", cronCount)
	}

	// Verify current-month partitions exist for partitioned tables.
	expectedCurrentPartitions := []string{"emails", "email_events", "audit_logs"}
	for _, base := range expectedCurrentPartitions {
		var exists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				  FROM information_schema.tables
				 WHERE table_schema = 'public'
				   AND table_name = $1 || '_' || to_char(date_trunc('month', now()), 'YYYY_MM')
			)
		`, base).Scan(&exists)
		if err != nil {
			t.Errorf("checking current-month partition for %s: %v", base, err)
			continue
		}
		if !exists {
			t.Errorf("expected current-month partition for %s to exist after migration up", base)
		}
	}

	pool.Close()

	// Run all migrations down
	if err := pgadapter.RunMigrationsDown(connStr, migrationsPath()); err != nil {
		t.Fatalf("RunMigrationsDown() error: %v", err)
	}

	// Reconnect and verify tables are gone
	pool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("reconnecting after down: %v", err)
	}
	defer pool.Close()

	for _, table := range expectedTables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Errorf("checking table %s after down: %v", table, err)
			continue
		}
		if exists {
			t.Errorf("expected table %s to NOT exist after migration down", table)
		}
	}
}
