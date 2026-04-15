package river

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

func BenchmarkSendWorker_RateLimitBurstSize(b *testing.B) {
	burstSizes := []int{1, 2, 4, 8}

	for _, burstSize := range burstSizes {
		b.Run("burst_"+strconv.Itoa(burstSize), func(b *testing.B) {
			rateLimiter := &mockRateLimiter{
				acquireBurstFn: func(_ context.Context, _ uuid.UUID, requested int) (int, error) {
					return requested, nil
				},
			}
			worker := &SendWorker{
				rateLimiter:        rateLimiter,
				rateLimitBurstSize: burstSize,
			}
			adapterID := uuid.Must(uuid.NewV7())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				allowed, err := worker.acquireRateLimitToken(context.Background(), adapterID)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
				if !allowed {
					b.Fatal("expected token acquisition to succeed")
				}
			}
			b.StopTimer()

			b.ReportMetric(float64(len(rateLimiter.acquireBurstCalls))/float64(b.N), "burst_acquires/op")
		})
	}
}
