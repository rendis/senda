package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// JobQueue manages background job processing.
type JobQueue interface {
	// EnqueueSend enqueues an email send job.
	EnqueueSend(ctx context.Context, job *SendJob) error

	// EnqueueSendTx enqueues an email send job within an existing transaction.
	EnqueueSendTx(ctx context.Context, tx pgx.Tx, job *SendJob) error

	// EnqueueWebhook enqueues a webhook delivery.
	EnqueueWebhook(ctx context.Context, job *WebhookJob) error
}

// SendJob represents an email send job.
type SendJob struct {
	EmailID    uuid.UUID
	TrackingID string
	AdapterID  uuid.UUID
	Priority   int // 1 = highest
}

// WebhookJob represents a webhook delivery job.
type WebhookJob struct {
	WebhookID  uuid.UUID
	EventType  string
	Payload    []byte
	RetryCount int
}
