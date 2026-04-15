package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/port"
)

// SNSReplayRepo persists SNS replay keys and enforces the replay window.
type SNSReplayRepo struct {
	pool *pgxpool.Pool
}

// NewSNSReplayRepo creates a new SNSReplayRepo.
func NewSNSReplayRepo(pool *pgxpool.Pool) *SNSReplayRepo {
	return &SNSReplayRepo{pool: pool}
}

// Claim stores a replay key if the SNS message is new and still inside the replay window.
func (r *SNSReplayRepo) Claim(ctx context.Context, topicArn, messageID string, messageTimestamp time.Time, replayWindow time.Duration) (port.SNSReplayDecision, error) {
	topicArn = strings.TrimSpace(topicArn)
	messageID = strings.TrimSpace(messageID)
	if topicArn == "" {
		return "", fmt.Errorf("topic ARN is required")
	}
	if messageID == "" {
		return "", fmt.Errorf("message ID is required")
	}
	if messageTimestamp.IsZero() {
		return "", fmt.Errorf("message timestamp is required")
	}
	if replayWindow <= 0 {
		return "", fmt.Errorf("replay window must be positive")
	}

	now := time.Now().UTC()
	age := now.Sub(messageTimestamp)
	if age > replayWindow || age < -replayWindow {
		return port.SNSReplayDecisionStale, nil
	}

	expiry := messageTimestamp.Add(replayWindow)
	if expiry.Before(now) {
		return port.SNSReplayDecisionStale, nil
	}

	tag, err := r.pool.Exec(ctx,
		`INSERT INTO sns_message_replays (
			topic_arn, message_id, message_timestamp, recorded_at, expires_at
		) VALUES (@topic_arn, @message_id, @message_timestamp, @recorded_at, @expires_at)
		ON CONFLICT (topic_arn, message_id) DO NOTHING`,
		pgx.NamedArgs{
			"topic_arn":         topicArn,
			"message_id":        messageID,
			"message_timestamp": messageTimestamp.UTC(),
			"recorded_at":       now,
			"expires_at":        expiry.UTC(),
		},
	)
	if err != nil {
		return "", fmt.Errorf("claiming SNS replay: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return port.SNSReplayDecisionDuplicate, nil
	}

	return port.SNSReplayDecisionAccepted, nil
}
