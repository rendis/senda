//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
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

// startPostgres creates a PostgreSQL container with pg_cron support using
// the project's custom Dockerfile.
func startPostgres(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

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

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("getting container host: %v", err)
	}

	mappedPort, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("getting mapped port: %v", err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, host, mappedPort.Port(), dbName)

	return ctr, connStr
}

// setupTestDB starts a postgres container, runs migrations, and returns a pool.
// It registers cleanup to terminate the container and close the pool.
func setupTestDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctr, connStr := startPostgres(ctx, t)
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminating container: %v", err)
		}
	})

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
