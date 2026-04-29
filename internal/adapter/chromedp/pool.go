package chromedp

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	cdp "github.com/chromedp/chromedp"

	"github.com/rendis/senda/config"
)

// ErrPoolStopped is returned by Acquire after Stop has been called.
var ErrPoolStopped = errors.New("chromedp: pool stopped")

// Pool owns one long-running Chromium allocator and gates concurrent
// browser-context use behind a semaphore.
type Pool struct {
	cfg    config.ScreenshotConfig
	logger *slog.Logger

	semaphore chan struct{}

	mu          sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
	stopped     bool

	// testMode: when true, AllocatorContext bypasses the real Chromium.
	testMode bool
}

// New builds a production pool with a real chromedp ExecAllocator.
func New(cfg config.ScreenshotConfig, logger *slog.Logger) *Pool {
	return &Pool{
		cfg:       cfg,
		logger:    logger,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}
}

// NewTestPool builds a pool that does NOT spawn Chromium. For unit tests.
func NewTestPool(cfg config.ScreenshotConfig) *Pool {
	return &Pool{
		cfg:       cfg,
		logger:    slog.Default(),
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		testMode:  true,
	}
}

// Acquire blocks until a slot is free or ctx is done. The returned release
// function MUST be called exactly once when the caller finishes.
func (p *Pool) Acquire(ctx context.Context) (release func(), err error) {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return nil, ErrPoolStopped
	}
	p.mu.Unlock()

	select {
	case p.semaphore <- struct{}{}:
		return func() { <-p.semaphore }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// AllocatorContext returns a context backed by the singleton ExecAllocator.
// Lazily starts it on first call. Not used in test mode.
func (p *Pool) AllocatorContext(ctx context.Context) (context.Context, error) {
	if p.testMode {
		return ctx, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return nil, ErrPoolStopped
	}
	if p.allocCtx == nil {
		opts := append(cdp.DefaultExecAllocatorOptions[:],
			cdp.ExecPath(p.cfg.ChromiumPath),
			cdp.NoSandbox,
			cdp.DisableGPU,
			cdp.Headless,
			cdp.Flag("hide-scrollbars", true),
			cdp.Flag("disable-dev-shm-usage", true),
		)
		p.allocCtx, p.allocCancel = cdp.NewExecAllocator(context.Background(), opts...)
		p.logger.Info("chromedp allocator started", "chromium_path", p.cfg.ChromiumPath)
	}
	return p.allocCtx, nil
}

// Restart kills the current allocator and forces the next AllocatorContext
// call to spawn a fresh Chromium. Safe to call from the capturer after a
// fatal error is detected.
func (p *Pool) Restart() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.allocCancel != nil {
		p.allocCancel()
		p.allocCtx = nil
		p.allocCancel = nil
		p.logger.Warn("chromedp allocator restarted")
	}
}

// Stop disables Acquire and tears down Chromium. Waits for in-flight slots
// to drain before cancelling the allocator context.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return nil
	}
	p.stopped = true

	// Drain the semaphore by filling all slots, waiting until each in-flight
	// caller returns its slot or ctx fires.
	for i := 0; i < p.cfg.MaxConcurrent; i++ {
		select {
		case p.semaphore <- struct{}{}:
		case <-ctx.Done():
			goto teardown
		}
	}

teardown:
	if p.allocCancel != nil {
		p.allocCancel()
		p.allocCtx = nil
		p.allocCancel = nil
	}
	return nil
}

// IdleTicker returns the configured idle timeout. Reserved for a future
// watchdog that stops Chromium after a period of inactivity.
func (p *Pool) IdleTicker() time.Duration {
	return p.cfg.IdleTimeout
}
