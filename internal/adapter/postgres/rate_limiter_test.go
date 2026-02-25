//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
)

func setupRateLimiter(t *testing.T) (*pgadapter.ProviderRateLimiter, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	pool := setupTestDB(ctx, t)

	return pgadapter.NewProviderRateLimiter(pool), pool
}

// createTestAdapter inserts the minimal FK chain (tenant -> workspace -> adapter)
// and returns the adapter ID.
func createTestAdapter(t *testing.T, ctx context.Context, pool *pgxpool.Pool, rateLimit int) uuid.UUID {
	t.Helper()

	tenantID := uuid.New()
	workspaceID := uuid.New()
	adapterID := uuid.New()

	_, err := pool.Exec(ctx,
		"INSERT INTO tenants (id, code, name) VALUES ($1, $2, $3)",
		tenantID, "test-"+tenantID.String()[:8], "Test Tenant",
	)
	if err != nil {
		t.Fatalf("inserting tenant: %v", err)
	}

	_, err = pool.Exec(ctx,
		"INSERT INTO workspaces (id, tenant_id, code, name) VALUES ($1, $2, $3, $4)",
		workspaceID, tenantID, "ws-"+workspaceID.String()[:8], "Test Workspace",
	)
	if err != nil {
		t.Fatalf("inserting workspace: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO adapters (id, workspace_id, name, adapter_type, config_encrypted, rate_limit_per_second)
		 VALUES ($1, $2, $3, 'ses', $4, $5)`,
		adapterID, workspaceID, "test-adapter", []byte("{}"), rateLimit,
	)
	if err != nil {
		t.Fatalf("inserting adapter: %v", err)
	}

	return adapterID
}

func TestProviderRateLimiter_TryAcquireAllows(t *testing.T) {
	rl, pool := setupRateLimiter(t)
	ctx := context.Background()

	adapterID := createTestAdapter(t, ctx, pool, 5)

	if err := rl.SyncBucket(ctx, adapterID, 5); err != nil {
		t.Fatalf("SyncBucket() error: %v", err)
	}

	for i := range 5 {
		allowed, err := rl.TryAcquire(ctx, adapterID)
		if err != nil {
			t.Fatalf("TryAcquire() #%d error: %v", i, err)
		}
		if !allowed {
			t.Errorf("TryAcquire() #%d = false, want true", i)
		}
	}
}

func TestProviderRateLimiter_TryAcquireExhausted(t *testing.T) {
	rl, pool := setupRateLimiter(t)
	ctx := context.Background()

	adapterID := createTestAdapter(t, ctx, pool, 3)

	if err := rl.SyncBucket(ctx, adapterID, 3); err != nil {
		t.Fatalf("SyncBucket() error: %v", err)
	}

	// Consume all tokens
	for i := range 3 {
		allowed, err := rl.TryAcquire(ctx, adapterID)
		if err != nil {
			t.Fatalf("TryAcquire() #%d error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("TryAcquire() #%d = false, want true (tokens not yet exhausted)", i)
		}
	}

	// Next attempt should be denied
	allowed, err := rl.TryAcquire(ctx, adapterID)
	if err != nil {
		t.Fatalf("TryAcquire() exhausted error: %v", err)
	}
	if allowed {
		t.Error("TryAcquire() after exhaustion = true, want false")
	}
}

func TestProviderRateLimiter_TokenRefill(t *testing.T) {
	rl, pool := setupRateLimiter(t)
	ctx := context.Background()

	adapterID := createTestAdapter(t, ctx, pool, 5)

	if err := rl.SyncBucket(ctx, adapterID, 5); err != nil {
		t.Fatalf("SyncBucket() error: %v", err)
	}

	// Consume all 5 tokens
	for i := range 5 {
		allowed, err := rl.TryAcquire(ctx, adapterID)
		if err != nil {
			t.Fatalf("TryAcquire() #%d error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("TryAcquire() #%d = false, want true", i)
		}
	}

	// Confirm exhausted
	allowed, err := rl.TryAcquire(ctx, adapterID)
	if err != nil {
		t.Fatalf("TryAcquire() exhausted error: %v", err)
	}
	if allowed {
		t.Fatal("TryAcquire() after exhaustion = true, want false")
	}

	// Wait for refill (at rate 5/sec, 1 second should refill 5 tokens)
	time.Sleep(1100 * time.Millisecond)

	allowed, err = rl.TryAcquire(ctx, adapterID)
	if err != nil {
		t.Fatalf("TryAcquire() after refill error: %v", err)
	}
	if !allowed {
		t.Error("TryAcquire() after refill = false, want true")
	}
}

func TestProviderRateLimiter_SyncBucketCreatesAndUpdates(t *testing.T) {
	rl, pool := setupRateLimiter(t)
	ctx := context.Background()

	adapterID := createTestAdapter(t, ctx, pool, 10)

	// Create bucket with rate=2
	if err := rl.SyncBucket(ctx, adapterID, 2); err != nil {
		t.Fatalf("SyncBucket(2) error: %v", err)
	}

	// Verify we can take exactly 2 tokens
	for i := range 2 {
		allowed, err := rl.TryAcquire(ctx, adapterID)
		if err != nil {
			t.Fatalf("TryAcquire() #%d error: %v", i, err)
		}
		if !allowed {
			t.Errorf("TryAcquire() #%d = false, want true", i)
		}
	}

	allowed, err := rl.TryAcquire(ctx, adapterID)
	if err != nil {
		t.Fatalf("TryAcquire() after 2 error: %v", err)
	}
	if allowed {
		t.Error("TryAcquire() #3 with rate=2 = true, want false")
	}

	// Update bucket to rate=5 — wait for refill, then sync
	time.Sleep(1100 * time.Millisecond)
	if err := rl.SyncBucket(ctx, adapterID, 5); err != nil {
		t.Fatalf("SyncBucket(5) error: %v", err)
	}

	// Verify max_tokens is now 5 by checking we can read the updated values
	var maxTokens int
	err = pool.QueryRow(ctx,
		"SELECT max_tokens FROM token_buckets WHERE adapter_id = $1",
		adapterID,
	).Scan(&maxTokens)
	if err != nil {
		t.Fatalf("querying max_tokens: %v", err)
	}
	if maxTokens != 5 {
		t.Errorf("max_tokens = %d, want 5", maxTokens)
	}
}
