package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
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

	migrations, err := LoadMigrations(os.DirFS("../.."), "db/migrations")
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if _, err := ApplyMigrations(ctx, pool, migrations, time.Now().UTC()); err != nil {
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
		RequestHash:     "request_hash_" + now.Format("150405000000000"),
		CorrelationID:   "corr_integration",
		AmountCents:     12345,
		Currency:        "BRL",
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: rail.KeyEmail,
		ReceiverName:    "Receiver",
		ReceiverBank:    "12345678",
		ReceiverRisk:    10,
		FraudScore:      10,
		FraudRules:      []string{},
		Status:          rail.StatusAccepted,
		DecisionReason:  "ok",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	event, err := events.New("evt_"+now.Format("150405000000000"), "pix_transfer_accepted", transfer.TenantID, transfer.AccountID, transfer.ID, transfer.CorrelationID, now, map[string]string{"status": "accepted"})
	if err != nil {
		t.Fatalf("event: %v", err)
	}

	if err := store.InsertTransfer(ctx, transfer, []events.Event{event}, []rail.AuditRecord{{
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
	idempotentTransfer, ok, err := store.FindByIdempotency(ctx, transfer.TenantID, transfer.IdempotencyKey)
	if err != nil {
		t.Fatalf("find idempotency: %v", err)
	}
	if !ok || idempotentTransfer.ID != transfer.ID {
		t.Fatalf("expected replay transfer, got ok=%v transfer=%+v", ok, idempotentTransfer)
	}
	pending, err := store.PendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("pending outbox: %v", err)
	}
	if len(pending) == 0 {
		t.Fatal("expected pending outbox event")
	}
	pendingSPI, err := store.PendingSPISubmissions(ctx, 10)
	if err != nil {
		t.Fatalf("pending spi: %v", err)
	}
	if len(pendingSPI) == 0 {
		t.Fatal("expected pending spi submission")
	}
	claimedSPI, replay, err := store.ClaimSPISubmission(ctx, transfer.TenantID, transfer.ID, "claim_"+now.Format("150405000000000"), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("claim spi: %v", err)
	}
	if replay || claimedSPI.SPIClaimToken == "" || claimedSPI.SPISubmissionAttempts != 1 {
		t.Fatalf("expected claimed spi transfer, replay=%v transfer=%+v", replay, claimedSPI)
	}

	message := rail.SPIMessage{
		MessageID:   "spi_" + now.Format("150405000000000"),
		EndToEndID:  "E" + now.Format("20060102150405"),
		TransferID:  transfer.ID,
		SubmittedAt: now.Add(time.Second),
	}
	spiEvent, err := events.New("evt_spi_"+now.Format("150405000000000"), "spi_message_created", transfer.TenantID, transfer.AccountID, transfer.ID, transfer.CorrelationID, now, map[string]string{"spi_message_id": message.MessageID})
	if err != nil {
		t.Fatalf("spi event: %v", err)
	}
	approved, replay, err := store.RecordSPISubmission(ctx, transfer.TenantID, transfer.ID, claimedSPI.SPIClaimToken, message, []events.Event{spiEvent}, rail.AuditRecord{
		TenantID:      transfer.TenantID,
		AccountID:     transfer.AccountID,
		TransferID:    transfer.ID,
		Action:        "spi_submission_recorded",
		CorrelationID: transfer.CorrelationID,
		Metadata:      map[string]string{"spi_message_id": message.MessageID},
		CreatedAt:     now,
	})
	if err != nil {
		t.Fatalf("record spi: %v", err)
	}
	if replay || approved.Status != rail.StatusApproved || approved.SPIMessageID != message.MessageID {
		t.Fatalf("expected approved spi submission, replay=%v transfer=%+v", replay, approved)
	}

	callback := rail.SettlementCallback{
		TenantID:     transfer.TenantID,
		TransferID:   transfer.ID,
		SPIMessageID: message.MessageID,
		Status:       rail.SettlementAccepted,
		Code:         "ACSC",
		ReceivedAt:   now.Add(2 * time.Second),
	}
	callback.CallbackHash = callback.Fingerprint()
	settlementEvent, err := events.New("evt_settlement_"+now.Format("150405000000000"), "pix_transfer_settled", transfer.TenantID, transfer.AccountID, transfer.ID, transfer.CorrelationID, now, map[string]string{"status": "settled"})
	if err != nil {
		t.Fatalf("settlement event: %v", err)
	}
	settled, replay, err := store.UpdateSettlement(ctx, transfer.TenantID, transfer.ID, callback, []events.Event{settlementEvent}, rail.AuditRecord{
		TenantID:      transfer.TenantID,
		AccountID:     transfer.AccountID,
		TransferID:    transfer.ID,
		Action:        "spi_settlement_callback_recorded",
		CorrelationID: transfer.CorrelationID,
		Metadata:      map[string]string{"status": "accepted"},
		CreatedAt:     now,
	})
	if err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if replay || settled.Status != rail.StatusSettled {
		t.Fatalf("expected settled transfer, replay=%v transfer=%+v", replay, settled)
	}
	_, replay, err = store.UpdateSettlement(ctx, transfer.TenantID, transfer.ID, callback, nil, rail.AuditRecord{})
	if err != nil {
		t.Fatalf("settlement replay: %v", err)
	}
	if !replay {
		t.Fatal("expected settlement callback hash replay")
	}
}
