//go:build integration

package pgcache_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/senda-app/senda/internal/adapter/pgcache"
	"github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/senda-app/senda/pkg/apperr"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}

func migrationsPath() string {
	return filepath.Join(projectRoot(), "migrations")
}

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

func setupCache(t *testing.T) (*pgadapter.PGCache, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ctr, connStr := startPostgres(ctx, t)
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Logf("terminating container: %v", err)
		}
	})

	if err := postgres.RunMigrations(connStr, migrationsPath()); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pgadapter.NewPGCache(pool), pool
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling JSON: %v", err)
	}
	return b
}

func TestPGCache_SetGetRoundTrip(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	value := mustJSON(t, map[string]string{"hello": "world"})

	if err := cache.Set(ctx, "test-key", value, 10*time.Second); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	got, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	var expected, actual map[string]string
	if err := json.Unmarshal(value, &expected); err != nil {
		t.Fatalf("unmarshal expected: %v", err)
	}
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("unmarshal actual: %v", err)
	}
	if expected["hello"] != actual["hello"] {
		t.Errorf("Get() = %s, want %s", string(got), string(value))
	}
}

func TestPGCache_TTLExpiry(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	value := mustJSON(t, "ephemeral")

	if err := cache.Set(ctx, "ttl-key", value, 1*time.Second); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	time.Sleep(2 * time.Second)

	_, err := cache.Get(ctx, "ttl-key")
	if err == nil {
		t.Fatal("Get() expected error after TTL expiry, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Get() expected *apperr.AppError, got %T: %v", err, err)
	}
	if appErr.Code != 404 {
		t.Errorf("Get() error code = %d, want 404", appErr.Code)
	}
}

func TestPGCache_Overwrite(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	v1 := mustJSON(t, "first")
	v2 := mustJSON(t, "second")

	if err := cache.Set(ctx, "overwrite-key", v1, 10*time.Second); err != nil {
		t.Fatalf("Set() v1 error: %v", err)
	}
	if err := cache.Set(ctx, "overwrite-key", v2, 10*time.Second); err != nil {
		t.Fatalf("Set() v2 error: %v", err)
	}

	got, err := cache.Get(ctx, "overwrite-key")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	var actual string
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if actual != "second" {
		t.Errorf("Get() = %q, want %q", actual, "second")
	}
}

func TestPGCache_Delete(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	value := mustJSON(t, "to-delete")

	if err := cache.Set(ctx, "del-key", value, 10*time.Second); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	if err := cache.Delete(ctx, "del-key"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err := cache.Get(ctx, "del-key")
	if err == nil {
		t.Fatal("Get() expected error after Delete, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("Get() expected 404 AppError, got: %v", err)
	}
}

func TestPGCache_DeletePattern(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	keys := map[string][]byte{
		"chain:a": mustJSON(t, "a"),
		"chain:b": mustJSON(t, "b"),
		"other:c": mustJSON(t, "c"),
	}

	for k, v := range keys {
		if err := cache.Set(ctx, k, v, 10*time.Second); err != nil {
			t.Fatalf("Set(%s) error: %v", k, err)
		}
	}

	if err := cache.DeletePattern(ctx, "chain:*"); err != nil {
		t.Fatalf("DeletePattern() error: %v", err)
	}

	// chain:a and chain:b should be gone
	for _, k := range []string{"chain:a", "chain:b"} {
		_, err := cache.Get(ctx, k)
		if err == nil {
			t.Errorf("Get(%s) expected error after DeletePattern, got nil", k)
		}
	}

	// other:c should still exist
	got, err := cache.Get(ctx, "other:c")
	if err != nil {
		t.Fatalf("Get(other:c) error: %v", err)
	}
	var actual string
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if actual != "c" {
		t.Errorf("Get(other:c) = %q, want %q", actual, "c")
	}
}

func TestPGCache_GetMiss(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	_, err := cache.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent key, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Get() expected *apperr.AppError, got %T: %v", err, err)
	}
	if appErr.Code != 404 {
		t.Errorf("Get() error code = %d, want 404", appErr.Code)
	}
}

func TestPGCache_StartCleanup(t *testing.T) {
	cache, _ := setupCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	value := mustJSON(t, "cleanup-me")

	if err := cache.Set(ctx, "cleanup-key", value, 1*time.Second); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	// Start cleanup with a short interval
	cache.StartCleanup(ctx, 500*time.Millisecond)

	// Wait for TTL to expire and cleanup to run
	time.Sleep(3 * time.Second)

	_, err := cache.Get(ctx, "cleanup-key")
	if err == nil {
		t.Fatal("Get() expected error after cleanup, got nil")
	}
	var appErr *apperr.AppError
	if !errors.As(err, &appErr) || appErr.Code != 404 {
		t.Errorf("Get() expected 404 AppError after cleanup, got: %v", err)
	}
}
