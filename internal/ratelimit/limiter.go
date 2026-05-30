package ratelimit

import (
	"sync"
	"time"
)

type BucketConfig struct {
	Capacity     int
	RefillTokens int
	RefillEvery  time.Duration
}

type Limiter struct {
	mu      sync.Mutex
	config  BucketConfig
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	tokens int
	seenAt time.Time
}

func New(config BucketConfig) *Limiter {
	if config.Capacity <= 0 {
		config.Capacity = 60
	}
	if config.RefillTokens <= 0 {
		config.RefillTokens = config.Capacity
	}
	if config.RefillEvery <= 0 {
		config.RefillEvery = time.Minute
	}
	return &Limiter{
		config:  config,
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

func NewWithClock(config BucketConfig, now func() time.Time) *Limiter {
	limiter := New(config)
	limiter.now = now
	return limiter
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.config.Capacity - 1, seenAt: now}
		return true
	}

	elapsed := now.Sub(b.seenAt)
	if elapsed >= l.config.RefillEvery {
		periods := int(elapsed / l.config.RefillEvery)
		b.tokens += periods * l.config.RefillTokens
		if b.tokens > l.config.Capacity {
			b.tokens = l.config.Capacity
		}
		b.seenAt = b.seenAt.Add(time.Duration(periods) * l.config.RefillEvery)
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}
