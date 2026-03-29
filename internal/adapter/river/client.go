package river

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goriver "github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/senda-app/senda/internal/port"
)

// SendJobArgs are the args for the send email worker.
type SendJobArgs struct {
	EmailID    uuid.UUID `json:"email_id"`
	TrackingID string    `json:"tracking_id"`
	AdapterID  uuid.UUID `json:"adapter_id"`
	Priority   int       `json:"priority"`
}

func (SendJobArgs) Kind() string { return "send_email" }

func (a SendJobArgs) InsertOpts() goriver.InsertOpts {
	return goriver.InsertOpts{
		MaxAttempts: 5,
		Priority:    a.Priority,
		Queue:       "send",
	}
}

// WebhookJobArgs are the args for the webhook delivery worker.
type WebhookJobArgs struct {
	WebhookID  uuid.UUID `json:"webhook_id"`
	EventType  string    `json:"event_type"`
	Payload    []byte    `json:"payload"`
	RetryCount int       `json:"retry_count"`
}

func (WebhookJobArgs) Kind() string { return "deliver_webhook" }

func (WebhookJobArgs) InsertOpts() goriver.InsertOpts {
	return goriver.InsertOpts{
		MaxAttempts: 6, // 1 initial + 5 retries
		Queue:       "webhook",
	}
}

// Client wraps a River client and implements port.JobQueue.
type Client struct {
	inner *goriver.Client[pgx.Tx]
	pool  *pgxpool.Pool
}

// NewClient creates a new River client with workers registered.
// Workers must be added before calling Start.
func NewClient(pool *pgxpool.Pool, sendWorker *SendWorker, webhookWorker *WebhookWorker) (*Client, error) {
	workers := goriver.NewWorkers()
	goriver.AddWorker(workers, sendWorker)
	goriver.AddWorker(workers, webhookWorker)

	client, err := goriver.NewClient(riverpgxv5.New(pool), &goriver.Config{
		Queues: map[string]goriver.QueueConfig{
			"send":    {MaxWorkers: 30},
			"webhook": {MaxWorkers: 15},
		},
		Workers:      workers,
		ErrorHandler: &errorHandler{},
	})
	if err != nil {
		return nil, fmt.Errorf("river: new client: %w", err)
	}

	return &Client{inner: client, pool: pool}, nil
}

// Start begins processing jobs. Blocks until ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	return c.inner.Start(ctx)
}

// Stop gracefully shuts down the River client.
func (c *Client) Stop(ctx context.Context) error {
	return c.inner.Stop(ctx)
}

// EnqueueSend enqueues an email send job.
func (c *Client) EnqueueSend(ctx context.Context, job *port.SendJob) error {
	priority := job.Priority
	if priority < 1 {
		priority = 2 // default priority
	}

	_, err := c.inner.Insert(ctx, SendJobArgs{
		EmailID:    job.EmailID,
		TrackingID: job.TrackingID,
		AdapterID:  job.AdapterID,
		Priority:   priority,
	}, nil)
	if err != nil {
		return fmt.Errorf("river: enqueue send: %w", err)
	}
	return nil
}

// EnqueueSendTx enqueues an email send job within an existing transaction.
func (c *Client) EnqueueSendTx(ctx context.Context, tx pgx.Tx, job *port.SendJob) error {
	priority := job.Priority
	if priority < 1 {
		priority = 2
	}

	_, err := c.inner.InsertTx(ctx, tx, SendJobArgs{
		EmailID:    job.EmailID,
		TrackingID: job.TrackingID,
		AdapterID:  job.AdapterID,
		Priority:   priority,
	}, nil)
	if err != nil {
		return fmt.Errorf("river: enqueue send tx: %w", err)
	}
	return nil
}

// EnqueueWebhook enqueues a webhook delivery job.
func (c *Client) EnqueueWebhook(ctx context.Context, job *port.WebhookJob) error {
	_, err := c.inner.Insert(ctx, WebhookJobArgs{
		WebhookID:  job.WebhookID,
		EventType:  job.EventType,
		Payload:    job.Payload,
		RetryCount: job.RetryCount,
	}, nil)
	if err != nil {
		return fmt.Errorf("river: enqueue webhook: %w", err)
	}
	return nil
}

// Healthy reports whether the River client is operational.
// It verifies that the underlying connection pool can reach PostgreSQL
// (which is also the River job store).
func (c *Client) Healthy(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Compile-time interface check.
var _ port.JobQueue = (*Client)(nil)

// errorHandler implements river.ErrorHandler for logging.
type errorHandler struct{}

func (h *errorHandler) HandleError(_ context.Context, job *rivertype.JobRow, err error) *goriver.ErrorHandlerResult {
	slog.Error("river: job failed", "kind", job.Kind, "id", job.ID, "attempt", job.Attempt, "error", err)

	// DLQ logging: when a job exhausts all retries, log a warning with identifiers
	// for manual inspection. River will discard the job after MaxAttempts.
	if job.Attempt >= job.MaxAttempts {
		attrs := []any{
			"kind", job.Kind,
			"job_id", job.ID,
			"attempts", job.Attempt,
			"max_attempts", job.MaxAttempts,
			"error", err,
		}
		// Try to extract email-specific identifiers from the job args.
		if job.Kind == "send_email" {
			var args struct {
				EmailID    string `json:"email_id"`
				TrackingID string `json:"tracking_id"`
			}
			if json.Unmarshal(job.EncodedArgs, &args) == nil {
				attrs = append(attrs, "email_id", args.EmailID, "tracking_id", args.TrackingID)
			}
		}
		slog.Warn("river: job exhausted all retries, entering dead letter state", attrs...)
	}

	return nil
}

func (h *errorHandler) HandlePanic(_ context.Context, job *rivertype.JobRow, panicVal any, trace string) *goriver.ErrorHandlerResult {
	slog.Error("river: job panicked", "kind", job.Kind, "id", job.ID, "panic", panicVal, "trace", trace)

	if job.Attempt >= job.MaxAttempts {
		attrs := []any{
			"kind", job.Kind,
			"job_id", job.ID,
			"attempts", job.Attempt,
			"max_attempts", job.MaxAttempts,
			"panic", panicVal,
		}
		if job.Kind == "send_email" {
			var args struct {
				EmailID    string `json:"email_id"`
				TrackingID string `json:"tracking_id"`
			}
			if json.Unmarshal(job.EncodedArgs, &args) == nil {
				attrs = append(attrs, "email_id", args.EmailID, "tracking_id", args.TrackingID)
			}
		}
		slog.Warn("river: job exhausted all retries (panic), entering dead letter state", attrs...)
	}

	return nil
}
