package ratelimit

import (
	"sync"
	"time"
)

type Strategy interface {
	Allow(key string) bool
}

type FixedWindowConfig struct {
	Limit  int
	Window time.Duration
}

type FixedWindowLimiter struct {
	mu      sync.Mutex
	config  FixedWindowConfig
	windows map[string]fixedWindow
	now     func() time.Time
}

type fixedWindow struct {
	start time.Time
	count int
}

func NewFixedWindow(config FixedWindowConfig) *FixedWindowLimiter {
	if config.Limit <= 0 {
		config.Limit = 60
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	return &FixedWindowLimiter{
		config:  config,
		windows: make(map[string]fixedWindow),
		now:     time.Now,
	}
}

func NewFixedWindowWithClock(config FixedWindowConfig, now func() time.Time) *FixedWindowLimiter {
	limiter := NewFixedWindow(config)
	limiter.now = now
	return limiter
}

func (l *FixedWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	window := l.windows[key]
	if window.start.IsZero() || now.Sub(window.start) >= l.config.Window {
		l.windows[key] = fixedWindow{start: now, count: 1}
		return true
	}
	if window.count >= l.config.Limit {
		return false
	}
	window.count++
	l.windows[key] = window
	return true
}

type SlidingWindowConfig struct {
	Limit  int
	Window time.Duration
}

type SlidingWindowLimiter struct {
	mu     sync.Mutex
	config SlidingWindowConfig
	events map[string][]time.Time
	now    func() time.Time
}

func NewSlidingWindow(config SlidingWindowConfig) *SlidingWindowLimiter {
	if config.Limit <= 0 {
		config.Limit = 60
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	return &SlidingWindowLimiter{
		config: config,
		events: make(map[string][]time.Time),
		now:    time.Now,
	}
}

func NewSlidingWindowWithClock(config SlidingWindowConfig, now func() time.Time) *SlidingWindowLimiter {
	limiter := NewSlidingWindow(config)
	limiter.now = now
	return limiter
}

func (l *SlidingWindowLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	cutoff := now.Add(-l.config.Window)
	events := l.events[key]
	keepFrom := 0
	for keepFrom < len(events) && !events[keepFrom].After(cutoff) {
		keepFrom++
	}
	if keepFrom > 0 {
		copy(events, events[keepFrom:])
		events = events[:len(events)-keepFrom]
	}
	if len(events) >= l.config.Limit {
		l.events[key] = events
		return false
	}
	events = append(events, now)
	l.events[key] = events
	return true
}

type LeakyBucketConfig struct {
	Capacity  int
	LeakEvery time.Duration
}

type LeakyBucketLimiter struct {
	mu      sync.Mutex
	config  LeakyBucketConfig
	buckets map[string]leakyBucket
	now     func() time.Time
}

type leakyBucket struct {
	level  int
	seenAt time.Time
}

func NewLeakyBucket(config LeakyBucketConfig) *LeakyBucketLimiter {
	if config.Capacity <= 0 {
		config.Capacity = 60
	}
	if config.LeakEvery <= 0 {
		config.LeakEvery = time.Second
	}
	return &LeakyBucketLimiter{
		config:  config,
		buckets: make(map[string]leakyBucket),
		now:     time.Now,
	}
}

func NewLeakyBucketWithClock(config LeakyBucketConfig, now func() time.Time) *LeakyBucketLimiter {
	limiter := NewLeakyBucket(config)
	limiter.now = now
	return limiter
}

func (l *LeakyBucketLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	bucket := l.buckets[key]
	if bucket.seenAt.IsZero() {
		l.buckets[key] = leakyBucket{level: 1, seenAt: now}
		return true
	}
	if elapsed := now.Sub(bucket.seenAt); elapsed >= l.config.LeakEvery {
		leaked := int(elapsed / l.config.LeakEvery)
		bucket.level -= leaked
		if bucket.level < 0 {
			bucket.level = 0
		}
		bucket.seenAt = bucket.seenAt.Add(time.Duration(leaked) * l.config.LeakEvery)
	}
	if bucket.level >= l.config.Capacity {
		l.buckets[key] = bucket
		return false
	}
	bucket.level++
	l.buckets[key] = bucket
	return true
}
