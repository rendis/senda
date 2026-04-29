package chromedp_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rendis/senda/config"
	chromedpadapter "github.com/rendis/senda/internal/adapter/chromedp"
)

func TestPool_AcquireReleaseRespectsSemaphore(t *testing.T) {
	cfg := config.ScreenshotConfig{MaxConcurrent: 2}
	pool := chromedpadapter.NewTestPool(cfg) // test-only constructor: skips real allocator

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rel1, err := pool.Acquire(ctx)
	require.NoError(t, err)
	rel2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Third acquire must block, then time out.
	_, err = pool.Acquire(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	rel1()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	rel3, err := pool.Acquire(ctx2)
	require.NoError(t, err)
	rel3()
	rel2()
}

func TestPool_AcquireRejectsWhenStopped(t *testing.T) {
	cfg := config.ScreenshotConfig{MaxConcurrent: 1}
	pool := chromedpadapter.NewTestPool(cfg)
	require.NoError(t, pool.Stop(context.Background()))

	_, err := pool.Acquire(context.Background())
	require.ErrorIs(t, err, chromedpadapter.ErrPoolStopped)
}
