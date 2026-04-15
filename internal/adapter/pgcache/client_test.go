//go:build integration

package pgcache_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/rendis/senda/internal/adapter/pgcache"
	"github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/pkg/apperr"
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

func startPostgres(ctx context.Context, t testing.TB) (testcontainers.Container, string) {
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

func setupCache(t testing.TB) (*pgadapter.PGCache, *pgxpool.Pool) {
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

func mustJSON(t testing.TB, v any) []byte {
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

func TestPGCache_DeletePattern_RespectsLiteralWildcardCharacters(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	keys := map[string][]byte{
		"scope:a_b:one": mustJSON(t, "literal"),
		"scope:axb:one": mustJSON(t, "similar"),
		"scope:a_b:two": mustJSON(t, "literal-2"),
	}

	for k, v := range keys {
		if err := cache.Set(ctx, k, v, 10*time.Second); err != nil {
			t.Fatalf("Set(%s) error: %v", k, err)
		}
	}

	if err := cache.DeletePattern(ctx, "scope:a_b:*"); err != nil {
		t.Fatalf("DeletePattern() error: %v", err)
	}

	for _, k := range []string{"scope:a_b:one", "scope:a_b:two"} {
		_, err := cache.Get(ctx, k)
		if err == nil {
			t.Fatalf("Get(%s) expected error after DeletePattern, got nil", k)
		}
	}

	got, err := cache.Get(ctx, "scope:axb:one")
	if err != nil {
		t.Fatalf("Get(scope:axb:one) error: %v", err)
	}
	var actual string
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if actual != "similar" {
		t.Fatalf("Get(scope:axb:one) = %q, want %q", actual, "similar")
	}
}

func TestPGCache_DeleteResolvedTemplatesByWorkspace(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	keys := map[string][]byte{
		fmt.Sprintf("resolved_template:%s:welcome:_default", workspaceID):      mustJSON(t, "welcome"),
		fmt.Sprintf("resolved_template:%s:receipt:es", workspaceID):            mustJSON(t, "receipt"),
		fmt.Sprintf("resolved_template:%s:welcome:_default", otherWorkspaceID): mustJSON(t, "other-workspace"),
		"chain:keep-me": mustJSON(t, "chain"),
	}
	for k, v := range keys {
		if err := cache.Set(ctx, k, v, 10*time.Second); err != nil {
			t.Fatalf("Set(%s) error: %v", k, err)
		}
	}

	if err := cache.DeleteResolvedTemplatesByWorkspace(ctx, workspaceID); err != nil {
		t.Fatalf("DeleteResolvedTemplatesByWorkspace() error: %v", err)
	}

	for _, k := range []string{
		fmt.Sprintf("resolved_template:%s:welcome:_default", workspaceID),
		fmt.Sprintf("resolved_template:%s:receipt:es", workspaceID),
	} {
		if _, err := cache.Get(ctx, k); err == nil {
			t.Fatalf("expected %s to be deleted", k)
		}
	}
	if _, err := cache.Get(ctx, fmt.Sprintf("resolved_template:%s:welcome:_default", otherWorkspaceID)); err != nil {
		t.Fatalf("expected other workspace key to remain, got %v", err)
	}
	if _, err := cache.Get(ctx, "chain:keep-me"); err != nil {
		t.Fatalf("expected non-template key to remain, got %v", err)
	}
}

func TestPGCache_DeleteAllResolvedTemplates(t *testing.T) {
	cache, _ := setupCache(t)
	ctx := context.Background()

	keys := map[string][]byte{
		fmt.Sprintf("resolved_template:%s:welcome:_default", uuid.New()): mustJSON(t, "welcome"),
		fmt.Sprintf("resolved_template:%s:receipt:es", uuid.New()):       mustJSON(t, "receipt"),
		"adapter:keep-me": mustJSON(t, "adapter"),
	}
	for k, v := range keys {
		if err := cache.Set(ctx, k, v, 10*time.Second); err != nil {
			t.Fatalf("Set(%s) error: %v", k, err)
		}
	}

	if err := cache.DeleteAllResolvedTemplates(ctx); err != nil {
		t.Fatalf("DeleteAllResolvedTemplates() error: %v", err)
	}

	for k := range keys {
		if strings.HasPrefix(k, "resolved_template:") {
			if _, err := cache.Get(ctx, k); err == nil {
				t.Fatalf("expected %s to be deleted", k)
			}
		}
	}
	if _, err := cache.Get(ctx, "adapter:keep-me"); err != nil {
		t.Fatalf("expected non-template key to remain, got %v", err)
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

func BenchmarkPGCache_DeletePattern(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name    string
		pattern string
		keys    []string
	}{
		{
			name:    "plain_prefix",
			pattern: "chain:*",
			keys:    []string{"chain:a", "chain:b", "chain:c", "other:x"},
		},
		{
			name:    "escaped_prefix",
			pattern: "scope:a_b:*",
			keys:    []string{"scope:a_b:one", "scope:a_b:two", "scope:axb:one", "other:y"},
		},
	}

	for _, tt := range cases {
		b.Run(tt.name, func(b *testing.B) {
			cache, _ := setupCache(b)
			ctx := context.Background()

			seed := func() {
				b.Helper()
				for _, key := range tt.keys {
					if err := cache.Set(ctx, key, mustJSON(b, key), 10*time.Second); err != nil {
						b.Fatalf("Set(%s) error: %v", key, err)
					}
				}
			}

			seed()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := cache.DeletePattern(ctx, tt.pattern); err != nil {
					b.Fatalf("DeletePattern(%s) error: %v", tt.pattern, err)
				}
				b.StopTimer()
				seed()
				b.StartTimer()
			}
		})
	}
}

func BenchmarkPGCache_DeleteResolvedTemplatesByWorkspace(b *testing.B) {
	b.ReportAllocs()

	cache, _ := setupCache(b)
	ctx := context.Background()
	workspaceID := uuid.New()
	otherWorkspaceID := uuid.New()
	keys := []string{
		fmt.Sprintf("resolved_template:%s:welcome:_default", workspaceID),
		fmt.Sprintf("resolved_template:%s:receipt:es", workspaceID),
		fmt.Sprintf("resolved_template:%s:welcome:_default", otherWorkspaceID),
		"chain:keep-me",
	}

	seed := func() {
		b.Helper()
		for _, key := range keys {
			if err := cache.Set(ctx, key, mustJSON(b, key), 10*time.Second); err != nil {
				b.Fatalf("Set(%s) error: %v", key, err)
			}
		}
	}

	seed()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cache.DeleteResolvedTemplatesByWorkspace(ctx, workspaceID); err != nil {
			b.Fatalf("DeleteResolvedTemplatesByWorkspace() error: %v", err)
		}
		b.StopTimer()
		seed()
		b.StartTimer()
	}
}
