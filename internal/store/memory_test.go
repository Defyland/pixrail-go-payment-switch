package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

func TestMemoryStoreEnforcesIdempotencyPerTenant(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	transfer := rail.Transfer{ID: "pxt_1", TenantID: "tenant_a", AccountID: "acct_1", IdempotencyKey: "idem", Status: rail.StatusApproved, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.InsertTransfer(ctx, transfer, nil, nil); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	if replay, ok := store.FindByIdempotency(ctx, "tenant_a", "idem"); !ok || replay.ID != transfer.ID {
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

	updated, err := store.UpdateSettlement(ctx, "tenant_a", "pxt_1", rail.SettlementCallback{
		TransferID:   "pxt_1",
		SPIMessageID: "spi_1",
		Status:       rail.SettlementAccepted,
		Code:         "ACSC",
		ReceivedAt:   time.Now().UTC(),
	}, nil, AuditRecord{})
	if err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	if updated.Status != rail.StatusSettled {
		t.Fatalf("expected settled, got %s", updated.Status)
	}

	replayed, err := store.UpdateSettlement(ctx, "tenant_a", "pxt_1", rail.SettlementCallback{
		TransferID:   "pxt_1",
		SPIMessageID: "spi_1",
		Status:       rail.SettlementRejected,
		Code:         "RJCT",
		ReceivedAt:   time.Now().UTC(),
	}, nil, AuditRecord{})
	if err != nil {
		t.Fatalf("terminal replay failed: %v", err)
	}
	if replayed.Status != rail.StatusSettled {
		t.Fatalf("terminal state should not change, got %s", replayed.Status)
	}
}
