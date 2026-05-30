package switcher

import (
	"context"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/dict"
	"github.com/Defyland/pixrail-go-payment-switch/internal/fraud"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/ratelimit"
	"github.com/Defyland/pixrail-go-payment-switch/internal/spi"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
)

func TestCreateTransferApprovesAndPublishesOutbox(t *testing.T) {
	service := newTestService(10)
	req := validRequest()

	result, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if result.Transfer.Status != rail.StatusApproved {
		t.Fatalf("expected approved, got %s", result.Transfer.Status)
	}
	if result.Transfer.SPIMessageID == "" || result.Transfer.EndToEndID == "" {
		t.Fatalf("expected SPI identifiers: %+v", result.Transfer)
	}
	outbox, err := service.Outbox(context.Background())
	if err != nil {
		t.Fatalf("outbox failed: %v", err)
	}
	if len(outbox) < 5 {
		t.Fatalf("expected requested, dict, fraud, spi, approved events")
	}
}

func TestCreateTransferIsIdempotent(t *testing.T) {
	service := newTestService(10)
	req := validRequest()

	first, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	second, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	if !second.IdempotentReplay {
		t.Fatal("expected idempotent replay")
	}
	if second.Transfer.ID != first.Transfer.ID {
		t.Fatalf("expected same transfer, got %s and %s", first.Transfer.ID, second.Transfer.ID)
	}
	outbox, err := service.Outbox(context.Background())
	if err != nil {
		t.Fatalf("outbox failed: %v", err)
	}
	if got := len(outbox); got != len(first.Events) {
		t.Fatalf("idempotent replay should not append events, got %d", got)
	}
}

func TestCreateTransferBlocksHighRiskReceiver(t *testing.T) {
	service := newTestService(10)
	req := validRequest()
	req.ReceiverKey = "mule@example.com"

	result, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if result.Transfer.Status != rail.StatusBlocked {
		t.Fatalf("expected blocked, got %s", result.Transfer.Status)
	}
	if result.Transfer.SPIMessageID != "" {
		t.Fatal("blocked transfers must not create SPI messages")
	}
}

func TestCreateTransferRateLimit(t *testing.T) {
	service := newTestService(1)
	req := validRequest()
	if _, err := service.CreateTransfer(context.Background(), req); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	req.IdempotencyKey = "idem-2"
	req.ReceiverKey = "other@example.com"
	if _, err := service.CreateTransfer(context.Background(), req); err == nil {
		t.Fatal("expected rate limit error")
	}
}

func TestRecordSettlementIsIdempotentForTerminalTransfer(t *testing.T) {
	service := newTestService(10)
	result, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	callback := rail.SettlementCallback{
		TenantID:      result.Transfer.TenantID,
		TransferID:    result.Transfer.ID,
		SPIMessageID:  result.Transfer.SPIMessageID,
		Status:        rail.SettlementAccepted,
		Code:          "ACSC",
		CorrelationID: "corr-settle",
	}
	settled, err := service.RecordSettlement(context.Background(), callback)
	if err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	if settled.Transfer.Status != rail.StatusSettled {
		t.Fatalf("expected settled, got %s", settled.Transfer.Status)
	}

	replay, err := service.RecordSettlement(context.Background(), callback)
	if err != nil {
		t.Fatalf("settlement replay failed: %v", err)
	}
	if !replay.IdempotentReplay {
		t.Fatal("expected terminal settlement replay")
	}
}

func validRequest() rail.CreateTransferRequest {
	return rail.CreateTransferRequest{
		TenantID:        "tenant_a",
		AccountID:       "acct_123",
		IdempotencyKey:  "idem-1",
		CorrelationID:   "corr-1",
		AmountCents:     12_345,
		Currency:        "BRL",
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: rail.KeyEmail,
		RequestedAt:     time.Now().UTC(),
	}
}

func newTestService(capacity int) *Service {
	return NewService(
		store.NewMemoryStore(),
		dict.StaticResolver{},
		fraud.RulesEngine{},
		spi.Simulator{},
		ratelimit.New(ratelimit.BucketConfig{Capacity: capacity, RefillTokens: capacity, RefillEvery: time.Minute}),
		ratelimit.New(ratelimit.BucketConfig{Capacity: capacity, RefillTokens: capacity, RefillEvery: time.Minute}),
	)
}
