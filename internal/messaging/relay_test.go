package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
)

func TestRelayPublishesAndMarksOutboxRecords(t *testing.T) {
	ctx := context.Background()
	store := storeWithOutboxEvent(t)
	publisher := &InMemoryPublisher{}
	relay := NewRelay(store, publisher, RetryPolicy{InitialBackoff: time.Second, MaxBackoff: time.Minute})

	stats, err := relay.Drain(ctx, 10)
	if err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if stats.Scanned != 1 || stats.Published != 1 || stats.Retried != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(publisher.Events) != 1 || publisher.Events[0].CorrelationID != "corr_1" {
		t.Fatalf("expected correlation-preserving publish, got %+v", publisher.Events)
	}
	if records := store.Outbox(ctx); !records[0].Published || records[0].DispatchedAt == nil {
		t.Fatalf("expected published outbox record, got %+v", records[0])
	}
}

func TestRelaySchedulesRetryWhenPublishFails(t *testing.T) {
	ctx := context.Background()
	store := storeWithOutboxEvent(t)
	publisher := failingPublisher{err: errors.New("broker down")}
	relay := NewRelay(store, publisher, RetryPolicy{InitialBackoff: time.Second, MaxBackoff: time.Minute})

	stats, err := relay.Drain(ctx, 10)
	if err != nil {
		t.Fatalf("drain failed: %v", err)
	}
	if stats.Scanned != 1 || stats.Published != 0 || stats.Retried != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	record := store.Outbox(ctx)[0]
	if record.Published || record.Attempts != 1 || record.LastError != "broker down" {
		t.Fatalf("expected retry evidence, got %+v", record)
	}
	if !record.AvailableAt.After(time.Now().UTC()) {
		t.Fatalf("expected future retry time, got %s", record.AvailableAt)
	}
}

func TestRelayBackoffCapsAtMax(t *testing.T) {
	relay := NewRelay(store.NewMemoryStore(), &InMemoryPublisher{}, RetryPolicy{InitialBackoff: time.Second, MaxBackoff: 3 * time.Second})
	if got := relay.backoff(5); got != 3*time.Second {
		t.Fatalf("expected max backoff, got %s", got)
	}
}

type failingPublisher struct {
	err error
}

func (p failingPublisher) Publish(context.Context, events.Event) error {
	return p.err
}

func storeWithOutboxEvent(t *testing.T) *store.MemoryStore {
	t.Helper()
	ctx := context.Background()
	memory := store.NewMemoryStore()
	event, err := events.New("evt_1", "pix_transfer_requested", "tenant_a", "acct_1", "pxt_1", "corr_1", time.Now().UTC(), map[string]string{"ok": "true"})
	if err != nil {
		t.Fatalf("event build failed: %v", err)
	}
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem_1", Status: rail.StatusApproved}
	if err := memory.InsertTransfer(ctx, transfer, []events.Event{event}, nil); err != nil {
		t.Fatalf("insert transfer failed: %v", err)
	}
	return memory
}
