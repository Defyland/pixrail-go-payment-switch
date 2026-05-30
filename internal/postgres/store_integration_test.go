package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	storepkg "github.com/Defyland/pixrail-go-payment-switch/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStoreIntegration(t *testing.T) {
	dsn := os.Getenv("PIXRAIL_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set PIXRAIL_POSTGRES_TEST_DSN to run postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	raw, err := os.ReadFile("../../db/migrations/0001_pixrail_core.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	store, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	transfer := rail.Transfer{
		ID:              "pxt_integration_" + now.Format("150405000000000"),
		TenantID:        "tenant_integration",
		AccountID:       "acct_integration",
		IdempotencyKey:  "idem_" + now.Format("150405000000000"),
		CorrelationID:   "corr_integration",
		EndToEndID:      "E" + now.Format("20060102150405"),
		AmountCents:     12345,
		Currency:        "BRL",
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: rail.KeyEmail,
		ReceiverName:    "Receiver",
		ReceiverBank:    "12345678",
		ReceiverRisk:    10,
		FraudScore:      10,
		FraudRules:      []string{},
		Status:          rail.StatusApproved,
		DecisionReason:  "ok",
		SPIMessageID:    "spi_" + now.Format("150405000000000"),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	event, err := events.New("evt_"+now.Format("150405000000000"), "pix_transfer_approved", transfer.TenantID, transfer.AccountID, transfer.ID, transfer.CorrelationID, now, map[string]string{"status": "approved"})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if err := store.InsertTransfer(ctx, transfer, []events.Event{event}, []storepkg.AuditRecord{{
		TenantID:      transfer.TenantID,
		AccountID:     transfer.AccountID,
		TransferID:    transfer.ID,
		Action:        "integration_test",
		CorrelationID: transfer.CorrelationID,
		Metadata:      map[string]string{"source": "test"},
		CreatedAt:     now,
	}}); err != nil {
		t.Fatalf("insert transfer: %v", err)
	}
	replay, ok, err := store.FindByIdempotency(ctx, transfer.TenantID, transfer.IdempotencyKey)
	if err != nil {
		t.Fatalf("find idempotency: %v", err)
	}
	if !ok || replay.ID != transfer.ID {
		t.Fatalf("expected replay transfer, got ok=%v transfer=%+v", ok, replay)
	}
	pending, err := store.PendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("pending outbox: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("expected pending outbox event")
	}
}
