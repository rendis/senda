//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	sharedPostgresOnce sync.Once
	sharedPostgresCtr  testcontainers.Container
	sharedPostgresConn string
	sharedPostgresErr  error
	sharedDBMu         sync.Mutex
)

func TestMain(m *testing.M) {
	code := m.Run()

	if sharedPostgresCtr != nil {
		if err := testcontainers.TerminateContainer(sharedPostgresCtr); err != nil {
			fmt.Fprintf(os.Stderr, "terminating shared postgres container: %v\n", err)
		}
	}

	os.Exit(code)
}

// projectRoot returns the absolute path to the project root directory.
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}

func migrationsPath() string {
	return filepath.Join(projectRoot(), "migrations")
}

// startPostgres creates a PostgreSQL container with pg_cron support using
// the project's custom Dockerfile.
func startPostgres(ctx context.Context) (testcontainers.Container, string, error) {
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
		return nil, "", fmt.Errorf("starting postgres container: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("getting container host: %w", err)
	}

	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		return nil, "", fmt.Errorf("getting mapped port: %w", err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, host, mappedPort.Port(), dbName)

	return ctr, connStr, nil
}

func sharedConnStr(ctx context.Context, t *testing.T) string {
	t.Helper()

	sharedPostgresOnce.Do(func() {
		sharedPostgresCtr, sharedPostgresConn, sharedPostgresErr = startPostgres(ctx)
	})

	if sharedPostgresErr != nil {
		t.Fatalf("starting shared postgres container: %v", sharedPostgresErr)
	}

	return sharedPostgresConn
}

// setupTestDB reuses one Postgres container per package and resets schema per test.
func setupTestDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	sharedDBMu.Lock()
	t.Cleanup(func() {
		sharedDBMu.Unlock()
	})

	connStr := sharedConnStr(ctx, t)

	if err := pgadapter.RunMigrationsDown(connStr, migrationsPath()); err != nil {
		t.Fatalf("resetting migrations down: %v", err)
	}
	if err := pgadapter.RunMigrations(connStr, migrationsPath()); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pool
}
