package switcher

import (
	"context"
	"errors"
	"sync"
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

func TestCreateTransferOrchestratesThroughPorts(t *testing.T) {
	spiClient := panicSPIClient{t: t}
	service := NewService(
		store.NewMemoryStore(),
		fakeParticipantResolver{},
		fakeFraudScorer{decision: rail.FraudDecision{Score: 12, Status: rail.StatusAccepted, Rules: []string{"fake_rule"}, Reason: "accepted by fake port"}},
		spiClient,
		allowAllLimiter{},
		allowAllLimiter{},
	)

	result, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if result.Transfer.Status != rail.StatusAccepted || result.Transfer.ReceiverBank != "12345678" {
		t.Fatalf("expected use case to map port outputs into transfer, got %+v", result.Transfer)
	}
}

func TestSubmitToSPIClaimsBeforeCallingSPI(t *testing.T) {
	memory := store.NewMemoryStore()
	spiClient := &blockingSPIClient{entered: make(chan struct{}), release: make(chan struct{})}
	service := NewService(memory, dict.StaticResolver{}, fraud.RulesEngine{}, spiClient, nil, nil)
	created, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	firstErr := make(chan error, 1)
	go func() {
		_, submitErr := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi-1")
		firstErr <- submitErr
	}()
	<-spiClient.entered

	if _, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi-2"); !errors.Is(err, rail.ErrConflict) {
		t.Fatalf("expected active claim conflict, got %v", err)
	}
	close(spiClient.release)
	if err := <-firstErr; err != nil {
		t.Fatalf("first submit failed: %v", err)
	}
	if got := spiClient.callCount(); got != 1 {
		t.Fatalf("expected exactly one SPI side effect, got %d", got)
	}
}

func TestSubmitToSPIReleasesClaimAfterSPIError(t *testing.T) {
	memory := store.NewMemoryStore()
	service := NewService(memory, dict.StaticResolver{}, fraud.RulesEngine{}, failingSPIClient{err: errors.New("spi unavailable")}, nil, nil)
	created, err := service.CreateTransfer(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := service.SubmitToSPI(context.Background(), created.Transfer.TenantID, created.Transfer.ID, "corr-spi"); err == nil {
		t.Fatal("expected spi failure")
	}
	transfer, err := memory.GetTransfer(context.Background(), created.Transfer.TenantID, created.Transfer.ID)
	if err != nil {
		t.Fatalf("get transfer failed: %v", err)
	}
	if transfer.SPIClaimToken != "" || transfer.SPIClaimedUntil == nil || transfer.SPILastError == "" {
		t.Fatalf("expected released claim with retry evidence, got %+v", transfer)
	}
	pending, err := memory.PendingSPISubmissions(context.Background(), 10)
	if err != nil {
		t.Fatalf("pending spi failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("transfer should wait for retry lease, got %d pending", len(pending))
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

func newTestServiceWithFraud(fraudEngine FraudScorer, capacity ...int) *Service {
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

type fakeParticipantResolver struct{}

func (fakeParticipantResolver) Resolve(context.Context, string, string, rail.DictKeyType) (rail.DictEntry, error) {
	return rail.DictEntry{
		ReceiverID:  "dict_fake",
		Name:        "Recebedor Fake",
		BankISPB:    "12345678",
		AccountHash: "receiver_hash",
		RiskSignal:  12,
		ResolvedAt:  time.Now().UTC(),
	}, nil
}

type fakeFraudScorer struct {
	decision rail.FraudDecision
}

func (s fakeFraudScorer) Score(context.Context, rail.CreateTransferRequest, rail.DictEntry) (rail.FraudDecision, error) {
	return s.decision, nil
}

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(string) bool {
	return true
}

type panicSPIClient struct {
	t *testing.T
}

func (c panicSPIClient) Submit(context.Context, rail.Transfer) (rail.SPIMessage, error) {
	c.t.Fatal("CreateTransfer must not call SPI before durable persistence")
	return rail.SPIMessage{}, nil
}

type failingSPIClient struct {
	err error
}

func (c failingSPIClient) Submit(context.Context, rail.Transfer) (rail.SPIMessage, error) {
	return rail.SPIMessage{}, c.err
}

type blockingSPIClient struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (c *blockingSPIClient) Submit(ctx context.Context, transfer rail.Transfer) (rail.SPIMessage, error) {
	c.mu.Lock()
	c.calls++
	calls := c.calls
	c.mu.Unlock()
	if calls > 1 {
		return rail.SPIMessage{}, errors.New("duplicate SPI side effect")
	}
	close(c.entered)
	select {
	case <-ctx.Done():
		return rail.SPIMessage{}, ctx.Err()
	case <-c.release:
		return rail.SPIMessage{
			MessageID:   "spi_blocking_1",
			EndToEndID:  "E2026053100000000001",
			TransferID:  transfer.ID,
			SubmittedAt: time.Now().UTC(),
		}, nil
	}
}

func (c *blockingSPIClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}
