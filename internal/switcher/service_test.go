package switcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/dict"
	"github.com/Defyland/pixrail-go-payment-switch/internal/fraud"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/ratelimit"
	"github.com/Defyland/pixrail-go-payment-switch/internal/spi"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
)

func TestCreateTransferPersistsAcceptedBeforeSPISideEffect(t *testing.T) {
	service := NewService(
		store.NewMemoryStore(),
		dict.StaticResolver{},
		fraud.RulesEngine{},
		panicSPIClient{t: t},
		ratelimit.New(ratelimit.BucketConfig{Capacity: 10, RefillTokens: 10, RefillEvery: time.Minute}),
		ratelimit.New(ratelimit.BucketConfig{Capacity: 10, RefillTokens: 10, RefillEvery: time.Minute}),
	)
	req := validRequest()

	result, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if result.Transfer.Status != rail.StatusAccepted {
		t.Fatalf("expected accepted, got %s", result.Transfer.Status)
	}
	if result.Transfer.SPIMessageID != "" || result.Transfer.EndToEndID != "" {
		t.Fatalf("create must not submit to SPI before durable persistence: %+v", result.Transfer)
	}
	outbox, err := service.Outbox(context.Background())
	if err != nil {
		t.Fatalf("outbox failed: %v", err)
	}
	if len(outbox) < 5 {
		t.Fatalf("expected requested, dict, fraud, accepted, and spi-requested events")
	}
}

func TestSubmitToSPIApprovesPersistedTransfer(t *testing.T) {
	service := newTestService(10)
	created, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	submitted, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi")
	if err != nil {
		t.Fatalf("submit spi failed: %v", err)
	}
	if submitted.Transfer.Status != rail.StatusApproved {
		t.Fatalf("expected approved, got %s", submitted.Transfer.Status)
	}
	if submitted.Transfer.SPIMessageID == "" || submitted.Transfer.EndToEndID == "" {
		t.Fatalf("expected SPI identifiers: %+v", submitted.Transfer)
	}

	replay, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi")
	if err != nil {
		t.Fatalf("submit spi replay failed: %v", err)
	}
	if !replay.IdempotentReplay {
		t.Fatal("expected idempotent spi submission replay")
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

func TestCreateTransferRejectsIdempotencyPayloadMismatch(t *testing.T) {
	service := newTestService(10)
	req := validRequest()
	if _, err := service.CreateTransfer(context.Background(), req); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	req.AmountCents++
	if _, err := service.CreateTransfer(context.Background(), req); !errors.Is(err, rail.ErrConflict) {
		t.Fatalf("expected idempotency payload conflict, got %v", err)
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
	created, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	result, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi")
	if err != nil {
		t.Fatalf("submit spi failed: %v", err)
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

func TestRecordSettlementRejectsTerminalCallbackWithWrongSPIMessage(t *testing.T) {
	service := newTestService(10)
	created, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	submitted, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi")
	if err != nil {
		t.Fatalf("submit spi failed: %v", err)
	}
	callback := rail.SettlementCallback{
		TenantID:      submitted.Transfer.TenantID,
		TransferID:    submitted.Transfer.ID,
		SPIMessageID:  submitted.Transfer.SPIMessageID,
		Status:        rail.SettlementAccepted,
		Code:          "ACSC",
		CorrelationID: "corr-settle",
	}
	if _, err := service.RecordSettlement(context.Background(), callback); err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	callback.SPIMessageID = "spi_wrong"
	if _, err := service.RecordSettlement(context.Background(), callback); !errors.Is(err, rail.ErrConflict) {
		t.Fatalf("expected spi mismatch conflict, got %v", err)
	}
}

func TestReviewCanApproveTransferIntoSPIPendingState(t *testing.T) {
	service := newTestServiceWithFraud(reviewFraudEngine{})
	req := validRequest()
	req.IdempotencyKey = "idem-review"
	created, err := service.CreateTransfer(context.Background(), req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.Transfer.Status != rail.StatusReview {
		t.Fatalf("expected review, got %s", created.Transfer.Status)
	}
	reviewed, err := service.RecordReview(context.Background(), rail.ReviewDecisionRequest{
		TenantID:   created.Transfer.TenantID,
		TransferID: created.Transfer.ID,
		Decision:   rail.ReviewApprove,
		Reason:     "manual approval after analyst review",
	})
	if err != nil {
		t.Fatalf("review failed: %v", err)
	}
	if reviewed.Transfer.Status != rail.StatusAccepted {
		t.Fatalf("expected accepted after review approval, got %s", reviewed.Transfer.Status)
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
	return newTestServiceWithFraud(fraud.RulesEngine{}, capacity)
}

func newTestServiceWithFraud(fraudEngine fraud.Engine, capacity ...int) *Service {
	bucketCapacity := 10
	if len(capacity) > 0 {
		bucketCapacity = capacity[0]
	}
	return NewService(
		store.NewMemoryStore(),
		dict.StaticResolver{},
		fraudEngine,
		spi.Simulator{},
		ratelimit.New(ratelimit.BucketConfig{Capacity: bucketCapacity, RefillTokens: bucketCapacity, RefillEvery: time.Minute}),
		ratelimit.New(ratelimit.BucketConfig{Capacity: bucketCapacity, RefillTokens: bucketCapacity, RefillEvery: time.Minute}),
	)
}

type reviewFraudEngine struct{}

func (reviewFraudEngine) Score(context.Context, rail.CreateTransferRequest, rail.DictEntry) (rail.FraudDecision, error) {
	return rail.FraudDecision{
		Score:  70,
		Status: rail.StatusReview,
		Rules:  []string{"manual_review_fixture"},
		Reason: "manual review required",
	}, nil
}

type panicSPIClient struct {
	t *testing.T
}

func (c panicSPIClient) Submit(context.Context, rail.Transfer) (rail.SPIMessage, error) {
	c.t.Fatal("CreateTransfer must not call SPI before durable persistence")
	return rail.SPIMessage{}, nil
}
