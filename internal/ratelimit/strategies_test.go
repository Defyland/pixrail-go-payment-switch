package ratelimit

import (
	"testing"
	"time"
)

func TestFixedWindowLimiter(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowWithClock(FixedWindowConfig{Limit: 2, Window: time.Second}, func() time.Time {
		return now
	})

	if !limiter.Allow("tenant") || !limiter.Allow("tenant") {
		t.Fatal("expected first two requests inside fixed window")
	}
	if limiter.Allow("tenant") {
		t.Fatal("expected fixed window limit")
	}
	now = now.Add(time.Second)
	if !limiter.Allow("tenant") {
		t.Fatal("expected new fixed window to allow")
	}
}

func TestSlidingWindowLimiter(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	limiter := NewSlidingWindowWithClock(SlidingWindowConfig{Limit: 2, Window: time.Second}, func() time.Time {
		return now
	})

	if !limiter.Allow("dict:key") || !limiter.Allow("dict:key") {
		t.Fatal("expected first two requests inside sliding window")
	}
	now = now.Add(900 * time.Millisecond)
	if limiter.Allow("dict:key") {
		t.Fatal("expected sliding window to retain events before full expiry")
	}
	now = now.Add(101 * time.Millisecond)
	if !limiter.Allow("dict:key") {
		t.Fatal("expected oldest sliding event to expire")
	}
}

func TestLeakyBucketLimiter(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	limiter := NewLeakyBucketWithClock(LeakyBucketConfig{Capacity: 2, LeakEvery: time.Second}, func() time.Time {
		return now
	})

	if !limiter.Allow("spi-worker") || !limiter.Allow("spi-worker") {
		t.Fatal("expected leaky bucket capacity")
	}
	if limiter.Allow("spi-worker") {
		t.Fatal("expected full leaky bucket to reject")
	}
	now = now.Add(time.Second)
	if !limiter.Allow("spi-worker") {
		t.Fatal("expected one leaked slot")
	}
}
