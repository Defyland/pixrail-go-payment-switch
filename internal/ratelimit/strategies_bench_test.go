package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkRateLimitStrategies(b *testing.B) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	limit := 1_000_000_000
	cases := map[string]Strategy{
		"token_bucket":   NewWithClock(BucketConfig{Capacity: limit, RefillTokens: limit, RefillEvery: time.Second}, func() time.Time { return now }),
		"fixed_window":   NewFixedWindowWithClock(FixedWindowConfig{Limit: limit, Window: time.Second}, func() time.Time { return now }),
		"sliding_window": NewSlidingWindowWithClock(SlidingWindowConfig{Limit: limit, Window: time.Second}, func() time.Time { return now }),
		"leaky_bucket":   NewLeakyBucketWithClock(LeakyBucketConfig{Capacity: limit, LeakEvery: time.Second}, func() time.Time { return now }),
	}
	for name, limiter := range cases {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !limiter.Allow("tenant:acct") {
					b.Fatal("unexpected rate limit")
				}
			}
		})
	}
}

func BenchmarkRateLimitHighCardinality(b *testing.B) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	cases := map[string]Strategy{
		"token_bucket":   NewWithClock(BucketConfig{Capacity: 10, RefillTokens: 10, RefillEvery: time.Second}, func() time.Time { return now }),
		"fixed_window":   NewFixedWindowWithClock(FixedWindowConfig{Limit: 10, Window: time.Second}, func() time.Time { return now }),
		"sliding_window": NewSlidingWindowWithClock(SlidingWindowConfig{Limit: 10, Window: time.Second}, func() time.Time { return now }),
		"leaky_bucket":   NewLeakyBucketWithClock(LeakyBucketConfig{Capacity: 10, LeakEvery: time.Second}, func() time.Time { return now }),
	}
	for name, limiter := range cases {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = limiter.Allow(fmt.Sprintf("tenant:%d", i%10_000))
			}
		})
	}
}
