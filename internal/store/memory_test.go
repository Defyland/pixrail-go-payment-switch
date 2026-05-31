package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

func TestMemoryStoreEnforcesIdempotencyPerTenant(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.InsertTransfer(ctx, transfer, nil, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if replay, ok, err := store.FindByIdempotency(ctx, "tenant_a", "idem"); err != nil || !ok || replay.ID != transfer.ID {
		t.Fatalf("expected idempotent replay, got ok=%v transfer=%+v", ok, replay)
	}

	otherTenant := rail.Transfer{ID: "pxt_2", TenantID: "tenant_b", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.InsertTransfer(ctx, otherTenant, nil, nil); err != nil {
		t.Fatalf("same idempotency key in different tenant should be allowed: %v", err)
	}
}

func TestMemoryStoreTenantIsolation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved}
	if err := store.InsertTransfer(ctx, transfer, nil, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if _, err := store.GetTransfer(ctx, "tenant_b", "pxt_1"); !errors.Is(err, rail.ErrNotFound) {
		t.Fatalf("expected tenant isolation not found, got %v", err)
	}
}

func TestMemoryStoreUpdatesSettlementOnce(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved, SPIMessageID: "spi_1"}
	if err := store.InsertTransfer(ctx, transfer, nil, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	callback := rail.SettlementCallback{
		TenantID:     "tenant_a",
		TransferID:   "pxt_1",
		SPIMessageID: "spi_1",
		Status:       rail.SettlementAccepted,
		Code:         "ACSC",
		ReceivedAt:   time.Now().UTC(),
	}
	callback.CallbackHash = callback.Fingerprint()
	updated, replay, err := store.UpdateSettlement(ctx, "tenant_a", "pxt_1", callback, nil, AuditRecord{})
	if err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	if replay {
		t.Fatal("first settlement should not be a replay")
	}
	if updated.Status != rail.StatusSettled {
		t.Fatalf("expected settled, got %s", updated.Status)
	}

	replayed, replay, err := store.UpdateSettlement(ctx, "tenant_a", "pxt_1", callback, nil, AuditRecord{})
	if err != nil {
		t.Fatalf("terminal replay failed: %v", err)
	}
	if !replay {
		t.Fatal("expected same callback to replay")
	}
	if replayed.Status != rail.StatusSettled {
		t.Fatalf("terminal state should not change, got %s", replayed.Status)
	}

	conflicting := rail.SettlementCallback{
		TenantID:     "tenant_a",
		TransferID:   "pxt_1",
		SPIMessageID: "spi_1",
		Status:       rail.SettlementRejected,
		Code:         "RJCT",
		ReceivedAt:   time.Now().UTC(),
	}
	conflicting.CallbackHash = conflicting.Fingerprint()
	if _, _, err := store.UpdateSettlement(ctx, "tenant_a", "pxt_1", conflicting, nil, AuditRecord{}); !errors.Is(err, rail.ErrConflict) {
		t.Fatalf("expected conflicting callback to fail, got %v", err)
	}
}

func TestMemoryStoreRecordsSPISubmissionAfterAcceptedTransfer(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusAccepted, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.InsertTransfer(ctx, transfer, nil, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	updated, replay, err := store.RecordSPISubmission(ctx, "tenant_a", "pxt_1", rail.SPIMessage{
		MessageID:   "spi_1",
		EndToEndID:  "E123",
		SubmittedAt: time.Now().UTC(),
	}, nil, AuditRecord{CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("record spi failed: %v", err)
	}
	if replay {
		t.Fatal("first spi submission should not replay")
	}
	if updated.Status != rail.StatusApproved || updated.SPIMessageID != "spi_1" {
		t.Fatalf("expected approved with spi id, got %+v", updated)
	}

	replayed, replay, err := store.RecordSPISubmission(ctx, "tenant_a", "pxt_1", rail.SPIMessage{
		MessageID:   "spi_1",
		EndToEndID:  "E123",
		SubmittedAt: time.Now().UTC(),
	}, nil, AuditRecord{CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("replay spi failed: %v", err)
	}
	if !replay || replayed.Status != rail.StatusApproved {
		t.Fatalf("expected spi replay, got replay=%v transfer=%+v", replay, replayed)
	}
}

func TestMemoryStoreOutboxRelayState(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved}
	event := testEvent(t, "evt_1")
	if err := store.InsertTransfer(ctx, transfer, []events.Event{event}, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	pending, err := store.PendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("pending failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending event, got %d", len(pending))
	}

	retryAt := time.Now().UTC().Add(time.Hour)
	if err := store.MarkOutboxFailed(ctx, pending[0].Sequence, "broker unavailable", retryAt); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	pending, err = store.PendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("pending after failure failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("failed event should wait for retry, got %d pending", len(pending))
	}

	records, err := store.Outbox(ctx)
	if err != nil {
		t.Fatalf("outbox failed: %v", err)
	}
	if records[0].Attempts != 1 || records[0].LastError != "broker unavailable" {
		t.Fatalf("expected failure evidence, got %+v", records[0])
	}
	if err := store.MarkOutboxPublished(ctx, records[0].Sequence, time.Now().UTC()); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	records, err = store.Outbox(ctx)
	if err != nil {
		t.Fatalf("outbox failed: %v", err)
	}
	if !records[0].Published || records[0].DispatchedAt == nil || records[0].LastError != "" {
		t.Fatalf("expected published event, got %+v", records[0])
	}
}

func testEvent(t *testing.T, id string) events.Event {
	t.Helper()
	event, err := events.New(id, "pix_transfer_requested", "tenant_a", "acct_1", "pxt_1", "corr_1", time.Now().UTC(), map[string]string{"ok": "true"})
	if err != nil {
		t.Fatalf("event build failed: %v", err)
	}
	return event
}
