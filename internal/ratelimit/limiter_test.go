package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllowsCapacityAndRefills(t *testing.T) {
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	limiter := NewWithClock(BucketConfig{Capacity: 2, RefillTokens: 1, RefillEvery: time.Second}, func() time.Time {
		return now
	})

	if !limiter.Allow("tenant:account") {
		t.Fatal("first request should pass")
	}
	if !limiter.Allow("tenant:account") {
		t.Fatal("second request should pass")
	}
	if limiter.Allow("tenant:account") {
		t.Fatal("third request should be rate limited")
	}

	now = now.Add(time.Second)
	if !limiter.Allow("tenant:account") {
		t.Fatal("bucket should refill one token")
	}
}
