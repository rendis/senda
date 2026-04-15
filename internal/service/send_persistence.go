package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendis/senda/internal/domain"
	"github.com/rendis/senda/internal/port"
)

type SendPersistenceWriter struct {
	emailStore port.EmailStore
	queue      port.JobQueue
	pool       *pgxpool.Pool
}

func NewSendPersistenceWriter(emailStore port.EmailStore, queue port.JobQueue, pool *pgxpool.Pool) *SendPersistenceWriter {
	return &SendPersistenceWriter{
		emailStore: emailStore,
		queue:      queue,
		pool:       pool,
	}
}

func (w *SendPersistenceWriter) CreateQueued(ctx context.Context, email *domain.Email, trackingID string, adapterID uuid.UUID) error {
	sendJob := &port.SendJob{
		EmailID:    email.ID,
		TrackingID: trackingID,
		AdapterID:  adapterID,
	}

	now := time.Now().UTC()
	queuedEvent := &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.EventTypeQueued,
		OccurredAt: now,
		CreatedAt:  now,
	}

	if w.pool != nil {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		if err := w.emailStore.CreateTx(ctx, tx, email); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("create email: %w", err)
		}
		if err := w.queue.EnqueueSendTx(ctx, tx, sendJob); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("enqueue send: %w", err)
		}
		if err := w.emailStore.AddEventTx(ctx, tx, queuedEvent); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("add queued event: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	}

	if err := w.emailStore.Create(ctx, email); err != nil {
		return fmt.Errorf("create email: %w", err)
	}
	if err := w.emailStore.AddEvent(ctx, queuedEvent); err != nil {
		slog.Error("failed to add queued event", "email_id", email.ID, "error", err)
	}
	if err := w.queue.EnqueueSend(ctx, sendJob); err != nil {
		return fmt.Errorf("enqueue send: %w", err)
	}
	return nil
}

func (w *SendPersistenceWriter) CreateSuppressed(ctx context.Context, email *domain.Email, now time.Time, reason string) error {
	suppressionEvent := &domain.EmailEvent{
		ID:         uuid.Must(uuid.NewV7()),
		EmailID:    email.ID,
		EventType:  domain.EventTypeSuppressed,
		OccurredAt: now,
		Metadata:   map[string]any{"reason": reason},
		CreatedAt:  now,
	}

	if w.pool != nil {
		tx, err := w.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		if err := w.emailStore.CreateTx(ctx, tx, email); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("create suppressed email: %w", err)
		}
		if err := w.emailStore.AddEventTx(ctx, tx, suppressionEvent); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("add suppression event: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	}

	if err := w.emailStore.Create(ctx, email); err != nil {
		return err
	}
	if err := w.emailStore.AddEvent(ctx, suppressionEvent); err != nil {
		slog.Error("failed to add suppression event", "email_id", email.ID, "error", err)
	}
	return nil
}
