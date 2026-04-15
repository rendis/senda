//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/rendis/senda/internal/adapter/postgres"
	"github.com/rendis/senda/internal/port"
)

func TestSNSReplayRepo_ClaimAcceptedAndPersists(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := postgres.NewSNSReplayRepo(pool)

	topicArn := "arn:aws:sns:us-east-1:123456789012:SES-Events"
	messageID := "sns-msg-001"
	messageTimestamp := time.Now().UTC().Add(-5 * time.Minute)
	window := 15 * time.Minute

	decision, err := repo.Claim(ctx, topicArn, messageID, messageTimestamp, window)
	if err != nil {
		t.Fatalf("Claim() error: %v", err)
	}
	if decision != port.SNSReplayDecisionAccepted {
		t.Fatalf("Claim() decision = %q, want %q", decision, port.SNSReplayDecisionAccepted)
	}

	var rows int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM sns_message_replays WHERE topic_arn = $1 AND message_id = $2`,
		topicArn, messageID,
	).Scan(&rows)
	if err != nil {
		t.Fatalf("counting replay rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 replay row, got %d", rows)
	}
}

func TestSNSReplayRepo_ClaimDuplicateReturnsDuplicate(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := postgres.NewSNSReplayRepo(pool)

	topicArn := "arn:aws:sns:us-east-1:123456789012:SES-Events"
	messageID := "sns-msg-002"
	messageTimestamp := time.Now().UTC().Add(-5 * time.Minute)
	window := 15 * time.Minute

	decision, err := repo.Claim(ctx, topicArn, messageID, messageTimestamp, window)
	if err != nil {
		t.Fatalf("first Claim() error: %v", err)
	}
	if decision != port.SNSReplayDecisionAccepted {
		t.Fatalf("first Claim() decision = %q, want %q", decision, port.SNSReplayDecisionAccepted)
	}

	decision, err = repo.Claim(ctx, topicArn, messageID, messageTimestamp, window)
	if err != nil {
		t.Fatalf("second Claim() error: %v", err)
	}
	if decision != port.SNSReplayDecisionDuplicate {
		t.Fatalf("second Claim() decision = %q, want %q", decision, port.SNSReplayDecisionDuplicate)
	}

	var rows int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM sns_message_replays WHERE topic_arn = $1 AND message_id = $2`,
		topicArn, messageID,
	).Scan(&rows)
	if err != nil {
		t.Fatalf("counting replay rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 replay row after duplicate claim, got %d", rows)
	}
}

func TestSNSReplayRepo_ClaimStaleReturnsStale(t *testing.T) {
	ctx := context.Background()
	pool := setupTestDB(ctx, t)
	repo := postgres.NewSNSReplayRepo(pool)

	topicArn := "arn:aws:sns:us-east-1:123456789012:SES-Events"
	messageID := "sns-msg-003"
	messageTimestamp := time.Now().UTC().Add(-2 * time.Hour)
	window := 15 * time.Minute

	decision, err := repo.Claim(ctx, topicArn, messageID, messageTimestamp, window)
	if err != nil {
		t.Fatalf("Claim() error: %v", err)
	}
	if decision != port.SNSReplayDecisionStale {
		t.Fatalf("Claim() decision = %q, want %q", decision, port.SNSReplayDecisionStale)
	}

	var rows int
	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM sns_message_replays WHERE topic_arn = $1 AND message_id = $2`,
		topicArn, messageID,
	).Scan(&rows)
	if err != nil {
		t.Fatalf("counting replay rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 replay rows for stale claim, got %d", rows)
	}
}
