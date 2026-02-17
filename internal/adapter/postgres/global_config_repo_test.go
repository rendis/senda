//go:build integration

package postgres_test

import (
	"context"
	"math"
	"testing"

	pgadapter "github.com/senda-app/senda/internal/adapter/postgres"
	"github.com/senda-app/senda/internal/domain"
)

func TestGlobalConfigRepo_Get_SeedData(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewGlobalConfigRepo(pool)

	cfg, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	// Verify seed values from 000015_global_config.up.sql
	if cfg.DefaultRetryCount != 3 {
		t.Errorf("DefaultRetryCount: want 3, got %d", cfg.DefaultRetryCount)
	}
	if cfg.RetryBackoffBaseSeconds != 60 {
		t.Errorf("RetryBackoffBaseSeconds: want 60, got %d", cfg.RetryBackoffBaseSeconds)
	}
	if cfg.LogRetentionDays != 365 {
		t.Errorf("LogRetentionDays: want 365, got %d", cfg.LogRetentionDays)
	}
	if cfg.BounceAlertThresholdPercent != 5.0 {
		t.Errorf("BounceAlertThresholdPercent: want 5.0, got %f", cfg.BounceAlertThresholdPercent)
	}
	if cfg.ComplaintAlertThresholdPercent != 0.1 {
		t.Errorf("ComplaintAlertThresholdPercent: want 0.1, got %f", cfg.ComplaintAlertThresholdPercent)
	}
	if cfg.DomainRecheckIntervalHours != 24 {
		t.Errorf("DomainRecheckIntervalHours: want 24, got %d", cfg.DomainRecheckIntervalHours)
	}
	if cfg.OnboardingCompleted != false {
		t.Errorf("OnboardingCompleted: want false, got %t", cfg.OnboardingCompleted)
	}
	if cfg.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestGlobalConfigRepo_Upsert(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewGlobalConfigRepo(pool)

	// Read seed data first
	original, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	// Modify values
	updated := &domain.GlobalConfig{
		DefaultRetryCount:             5,
		RetryBackoffBaseSeconds:       120,
		LogRetentionDays:              180,
		BounceAlertThresholdPercent:   10.0,
		ComplaintAlertThresholdPercent: 0.5,
		DomainRecheckIntervalHours:    48,
		OnboardingCompleted:           true,
	}

	if err := repo.Upsert(ctx, updated); err != nil {
		t.Fatalf("Upsert() error: %v", err)
	}

	// Re-read and verify
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() after Upsert() error: %v", err)
	}

	if got.DefaultRetryCount != 5 {
		t.Errorf("DefaultRetryCount: want 5, got %d", got.DefaultRetryCount)
	}
	if got.RetryBackoffBaseSeconds != 120 {
		t.Errorf("RetryBackoffBaseSeconds: want 120, got %d", got.RetryBackoffBaseSeconds)
	}
	if got.LogRetentionDays != 180 {
		t.Errorf("LogRetentionDays: want 180, got %d", got.LogRetentionDays)
	}
	if math.Abs(got.BounceAlertThresholdPercent-10.0) > 0.001 {
		t.Errorf("BounceAlertThresholdPercent: want 10.0, got %f", got.BounceAlertThresholdPercent)
	}
	if math.Abs(got.ComplaintAlertThresholdPercent-0.5) > 0.001 {
		t.Errorf("ComplaintAlertThresholdPercent: want 0.5, got %f", got.ComplaintAlertThresholdPercent)
	}
	if got.DomainRecheckIntervalHours != 48 {
		t.Errorf("DomainRecheckIntervalHours: want 48, got %d", got.DomainRecheckIntervalHours)
	}
	if !got.OnboardingCompleted {
		t.Error("OnboardingCompleted: want true, got false")
	}
	if got.UpdatedAt.Before(original.UpdatedAt) {
		t.Error("expected UpdatedAt to advance after Upsert")
	}
}

func TestGlobalConfigRepo_Upsert_PartialUpdate(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := pgadapter.NewGlobalConfigRepo(pool)

	// Only change one field, rest should be overwritten with the values we pass
	cfg := &domain.GlobalConfig{
		DefaultRetryCount:             10,
		RetryBackoffBaseSeconds:       60,
		LogRetentionDays:              365,
		BounceAlertThresholdPercent:   5.0,
		ComplaintAlertThresholdPercent: 0.1,
		DomainRecheckIntervalHours:    24,
		OnboardingCompleted:           false,
	}

	if err := repo.Upsert(ctx, cfg); err != nil {
		t.Fatalf("Upsert() error: %v", err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if got.DefaultRetryCount != 10 {
		t.Errorf("DefaultRetryCount: want 10, got %d", got.DefaultRetryCount)
	}
	// Other fields should match what we passed
	if got.RetryBackoffBaseSeconds != 60 {
		t.Errorf("RetryBackoffBaseSeconds: want 60, got %d", got.RetryBackoffBaseSeconds)
	}
}
