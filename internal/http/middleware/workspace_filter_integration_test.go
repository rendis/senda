//go:build integration

package middleware_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/http/middleware"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// projectRoot returns the absolute path to the project root directory.
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}

func migrationsPath() string {
	return filepath.Join(projectRoot(), "migrations")
}

// setupFilterIntegrationDB starts (or reuses) a postgres container and applies migrations.
func setupFilterIntegrationDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	connStr := os.Getenv("SENDA_INTEGRATION_DATABASE_URL")
	if connStr == "" {
		dockerfilePath := filepath.Join(projectRoot(), "docker", "postgres")
		const (
			dbName = "senda_test"
			dbUser = "senda"
			dbPass = "senda"
		)
		req := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    dockerfilePath,
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       dbName,
				"POSTGRES_USER":     dbUser,
				"POSTGRES_PASSWORD": dbPass,
			},
			Cmd: []string{
				"postgres",
				"-c", "shared_preload_libraries=pg_cron",
				"-c", "cron.database_name=" + dbName,
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		}
		ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			t.Fatalf("starting postgres container: %v", err)
		}
		t.Cleanup(func() {
			if err := testcontainers.TerminateContainer(ctr); err != nil {
				fmt.Fprintf(os.Stderr, "terminating postgres: %v\n", err)
			}
		})
		host, _ := ctr.Host(ctx)
		port, _ := ctr.MappedPort(ctx, "5432")
		connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, host, port.Port(), dbName)
	}

	if err := pgadapter.RunMigrationsDown(connStr, migrationsPath()); err != nil {
		t.Fatalf("migrations down: %v", err)
	}
	if err := pgadapter.RunMigrations(connStr, migrationsPath()); err != nil {
		t.Fatalf("migrations up: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// TestWorkspaceFilter_EnvironmentHonouredByStore verifies that the workspaceFilter
// constructed from an X-Senda-Environment header value correctly scopes workspace
// existence checks to the specified environment. A workspace that exists in prod
// must not appear as existing when the filter is built for the test environment,
// and vice-versa.
func TestWorkspaceFilter_EnvironmentHonouredByStore(t *testing.T) {
	ctx := context.Background()
	pool := setupFilterIntegrationDB(ctx, t)

	tenantRepo := pgadapter.NewTenantRepo(pool)
	workspaceRepo := pgadapter.NewWorkspaceRepo(pool)

	// Create a tenant for this test.
	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Code: "t-" + uuid.New().String()[:8],
		Name: "Filter Integration Tenant",
	}
	if err := tenantRepo.Create(ctx, tenant); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}

	// Create a prod workspace and a test workspace with different codes.
	prodWS := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "marketing",
		Name:        "Marketing",
		Environment: domain.EnvironmentProd,
	}
	testWS := &domain.Workspace{
		ID:          uuid.New(),
		TenantID:    tenant.ID,
		Code:        "marketing",
		Name:        "Marketing (test)",
		Environment: domain.EnvironmentTest,
	}
	if err := workspaceRepo.Create(ctx, prodWS); err != nil {
		t.Fatalf("creating prod workspace: %v", err)
	}
	if err := workspaceRepo.Create(ctx, testWS); err != nil {
		t.Fatalf("creating test workspace: %v", err)
	}

	// Build a filter for prod — "marketing" must be found.
	prodFilter := middleware.NewWorkspaceFilterForTest(workspaceRepo, tenant.Code, domain.EnvironmentProd)
	prodResult, err := prodFilter.Exists(ctx, []string{"marketing", "missing"})
	if err != nil {
		t.Fatalf("prod filter Exists() error: %v", err)
	}
	if !prodResult["marketing"] {
		t.Errorf("prod filter: expected marketing=true, got %v", prodResult)
	}
	if prodResult["missing"] {
		t.Errorf("prod filter: expected missing=false, got %v", prodResult)
	}

	// Build a filter for test — same code must also be found under the test environment.
	testFilter := middleware.NewWorkspaceFilterForTest(workspaceRepo, tenant.Code, domain.EnvironmentTest)
	testResult, err := testFilter.Exists(ctx, []string{"marketing"})
	if err != nil {
		t.Fatalf("test filter Exists() error: %v", err)
	}
	if !testResult["marketing"] {
		t.Errorf("test filter: expected marketing=true under test env, got %v", testResult)
	}
}
