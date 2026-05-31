package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
)

type OutboxStore interface {
	ClaimPendingOutbox(ctx context.Context, limit int, claimToken string, claimUntil time.Time) ([]events.OutboxRecord, error)
	MarkOutboxPublished(ctx context.Context, sequence int64, claimToken string, dispatchedAt time.Time) error
	MarkOutboxFailed(ctx context.Context, sequence int64, claimToken string, lastError string, retryAt time.Time) error
}

type Publisher interface {
	Publish(ctx context.Context, event events.Event) error
}

type Relay struct {
	store     OutboxStore
	publisher Publisher
	policy    RetryPolicy
	now       func() time.Time
}

type RetryPolicy struct {
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	ClaimTTL       time.Duration
}

type RelayStats struct {
	Scanned   int
	Published int
	Retried   int
}

func NewRelay(store OutboxStore, publisher Publisher, policy RetryPolicy) *Relay {
	if policy.InitialBackoff <= 0 {
		policy.InitialBackoff = time.Second
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = time.Minute
	}
	if policy.ClaimTTL <= 0 {
		policy.ClaimTTL = 30 * time.Second
	}
	return &Relay{
		store:     store,
		publisher: publisher,
		policy:    policy,
		now:       time.Now,
	}
}

func (r *Relay) Drain(ctx context.Context, limit int) (RelayStats, error) {
	claimToken := newClaimToken("outbox")
	records, err := r.store.ClaimPendingOutbox(ctx, limit, claimToken, r.now().UTC().Add(r.policy.ClaimTTL))
	if err != nil {
		return RelayStats{}, err
	}

	stats := RelayStats{Scanned: len(records)}
	for _, record := range records {
		if err := r.publisher.Publish(ctx, record.Event); err != nil {
			stats.Retried++
			retryAt := r.now().UTC().Add(r.backoff(record.Attempts + 1))
			if markErr := r.store.MarkOutboxFailed(ctx, record.Sequence, claimToken, err.Error(), retryAt); markErr != nil {
				return stats, fmt.Errorf("mark outbox failed: %w", markErr)
			}
			continue
		}
		if err := r.store.MarkOutboxPublished(ctx, record.Sequence, claimToken, r.now().UTC()); err != nil {
			return stats, fmt.Errorf("mark outbox published: %w", err)
		}
		stats.Published++
	}
	return stats, nil
}

func newClaimToken(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func (r *Relay) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	backoff := r.policy.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= r.policy.MaxBackoff {
			return r.policy.MaxBackoff
		}
	}
	return backoff
}

type InMemoryPublisher struct {
	Events []events.Event
}

func (p *InMemoryPublisher) Publish(_ context.Context, event events.Event) error {
	p.Events = append(p.Events, event)
	return nil
}
