package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
)

type OutboxStore interface {
	PendingOutbox(ctx context.Context, limit int) ([]events.OutboxRecord, error)
	MarkOutboxPublished(ctx context.Context, sequence int64, dispatchedAt time.Time) error
	MarkOutboxFailed(ctx context.Context, sequence int64, lastError string, retryAt time.Time) error
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
	return &Relay{
		store:     store,
		publisher: publisher,
		policy:    policy,
		now:       time.Now,
	}
}

func (r *Relay) Drain(ctx context.Context, limit int) (RelayStats, error) {
	records, err := r.store.PendingOutbox(ctx, limit)
	if err != nil {
		return RelayStats{}, err
	}

	stats := RelayStats{Scanned: len(records)}
	for _, record := range records {
		if err := r.publisher.Publish(ctx, record.Event); err != nil {
			stats.Retried++
			retryAt := r.now().UTC().Add(r.backoff(record.Attempts + 1))
			if markErr := r.store.MarkOutboxFailed(ctx, record.Sequence, err.Error(), retryAt); markErr != nil {
				return stats, fmt.Errorf("mark outbox failed: %w", markErr)
			}
			continue
		}
		if err := r.store.MarkOutboxPublished(ctx, record.Sequence, r.now().UTC()); err != nil {
			return stats, fmt.Errorf("mark outbox published: %w", err)
		}
		stats.Published++
	}
	return stats, nil
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
