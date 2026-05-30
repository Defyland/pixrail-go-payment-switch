package switcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/dict"
	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/fraud"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/ratelimit"
	"github.com/Defyland/pixrail-go-payment-switch/internal/spi"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
)

type Store interface {
	Health(ctx context.Context) error
	FindByIdempotency(ctx context.Context, tenantID, key string) (rail.Transfer, bool, error)
	InsertTransfer(ctx context.Context, transfer rail.Transfer, outbox []events.Event, audit []store.AuditRecord) error
	GetTransfer(ctx context.Context, tenantID, transferID string) (rail.Transfer, error)
	UpdateSettlement(ctx context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit store.AuditRecord) (rail.Transfer, error)
	Outbox(ctx context.Context) []events.OutboxRecord
	Audit(ctx context.Context) []store.AuditRecord
}

type Service struct {
	store       Store
	dict        dict.Resolver
	fraud       fraud.Engine
	spi         spi.Client
	tenantLimit *ratelimit.Limiter
	dictLimit   *ratelimit.Limiter
	now         func() time.Time
}

type Result struct {
	Transfer         rail.Transfer
	IdempotentReplay bool
	Events           []events.Event
}

func NewService(store Store, dictResolver dict.Resolver, fraudEngine fraud.Engine, spiClient spi.Client, tenantLimit *ratelimit.Limiter, dictLimit *ratelimit.Limiter) *Service {
	return &Service{
		store:       store,
		dict:        dictResolver,
		fraud:       fraudEngine,
		spi:         spiClient,
		tenantLimit: tenantLimit,
		dictLimit:   dictLimit,
		now:         time.Now,
	}
}

func (s *Service) CreateTransfer(ctx context.Context, req rail.CreateTransferRequest) (Result, error) {
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.AccountID = strings.TrimSpace(req.AccountID)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	req.CorrelationID = strings.TrimSpace(req.CorrelationID)
	req.ReceiverKey = strings.TrimSpace(req.ReceiverKey)
	if req.Currency == "" {
		req.Currency = "BRL"
	}
	if req.RequestedAt.IsZero() {
		req.RequestedAt = s.now().UTC()
	}
	if err := req.Validate(); err != nil {
		return Result{}, err
	}

	if transfer, ok, err := s.store.FindByIdempotency(ctx, req.TenantID, req.IdempotencyKey); err != nil {
		return Result{}, err
	} else if ok {
		return Result{Transfer: transfer, IdempotentReplay: true}, nil
	}
	if s.tenantLimit != nil && !s.tenantLimit.Allow(req.TenantID+":"+req.AccountID) {
		return Result{}, fmt.Errorf("%w: tenant/account bucket exhausted", rail.ErrRateLimited)
	}
	if s.dictLimit != nil && !s.dictLimit.Allow(req.TenantID+":"+req.ReceiverKey) {
		return Result{}, fmt.Errorf("%w: dict lookup bucket exhausted", rail.ErrRateLimited)
	}

	dictEntry, err := s.dict.Resolve(ctx, req.TenantID, req.ReceiverKey, req.ReceiverKeyType)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return Result{}, fmt.Errorf("%w: dict lookup timeout", rail.ErrDependencyFailed)
		}
		return Result{}, err
	}

	decision, err := s.fraud.Score(ctx, req, dictEntry)
	if err != nil {
		return Result{}, err
	}

	now := s.now().UTC()
	transfer := rail.Transfer{
		ID:              newID("pxt"),
		TenantID:        req.TenantID,
		AccountID:       req.AccountID,
		IdempotencyKey:  req.IdempotencyKey,
		CorrelationID:   req.CorrelationID,
		AmountCents:     req.AmountCents,
		Currency:        req.Currency,
		ReceiverKey:     req.ReceiverKey,
		ReceiverKeyType: req.ReceiverKeyType,
		ReceiverName:    dictEntry.Name,
		ReceiverBank:    dictEntry.BankISPB,
		ReceiverRisk:    dictEntry.RiskSignal,
		FraudScore:      decision.Score,
		FraudRules:      append([]string{}, decision.Rules...),
		Status:          decision.Status,
		DecisionReason:  decision.Reason,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	outbox := make([]events.Event, 0, 5)
	addEvent := func(eventType string, payload any) {
		event, eventErr := events.New(newID("evt"), eventType, transfer.TenantID, transfer.AccountID, transfer.ID, transfer.CorrelationID, s.now(), payload)
		if eventErr == nil {
			outbox = append(outbox, event)
		}
	}

	addEvent("pix_transfer_requested", map[string]any{
		"amount_cents":      transfer.AmountCents,
		"currency":          transfer.Currency,
		"receiver_key_type": transfer.ReceiverKeyType,
	})
	addEvent("dict_key_resolved", map[string]any{
		"receiver_id":   dictEntry.ReceiverID,
		"receiver_bank": dictEntry.BankISPB,
		"risk_signal":   dictEntry.RiskSignal,
	})
	addEvent("fraud_score_calculated", map[string]any{
		"score":  decision.Score,
		"status": decision.Status,
		"rules":  decision.Rules,
	})

	if decision.Status == rail.StatusApproved {
		message, err := s.spi.Submit(ctx, transfer)
		if err != nil {
			return Result{}, err
		}
		transfer.SPIMessageID = message.MessageID
		transfer.EndToEndID = message.EndToEndID
		transfer.UpdatedAt = message.SubmittedAt
		addEvent("spi_message_created", map[string]any{
			"spi_message_id": message.MessageID,
			"end_to_end_id":  message.EndToEndID,
		})
		addEvent("pix_transfer_approved", map[string]any{
			"end_to_end_id":   message.EndToEndID,
			"spi_message_id":  message.MessageID,
			"decision_reason": transfer.DecisionReason,
		})
	} else if decision.Status == rail.StatusBlocked {
		addEvent("pix_transfer_blocked", map[string]any{
			"score":           decision.Score,
			"rules":           decision.Rules,
			"decision_reason": decision.Reason,
		})
	}

	audit := []store.AuditRecord{{
		TenantID:      transfer.TenantID,
		AccountID:     transfer.AccountID,
		TransferID:    transfer.ID,
		Action:        "pix_transfer_decided",
		CorrelationID: transfer.CorrelationID,
		Metadata: map[string]string{
			"status": string(transfer.Status),
			"score":  fmt.Sprintf("%d", transfer.FraudScore),
		},
		CreatedAt: now,
	}}
	if err := s.store.InsertTransfer(ctx, transfer, outbox, audit); err != nil {
		return Result{}, err
	}
	return Result{Transfer: transfer, Events: outbox}, nil
}

func (s *Service) GetTransfer(ctx context.Context, tenantID, transferID string) (rail.Transfer, error) {
	return s.store.GetTransfer(ctx, tenantID, transferID)
}

func (s *Service) Health(ctx context.Context) error {
	return s.store.Health(ctx)
}

func (s *Service) RecordSettlement(ctx context.Context, callback rail.SettlementCallback) (Result, error) {
	if callback.ReceivedAt.IsZero() {
		callback.ReceivedAt = s.now().UTC()
	}
	if callback.CorrelationID == "" {
		callback.CorrelationID = newID("corr")
	}
	if callback.TenantID == "" || callback.TransferID == "" || callback.SPIMessageID == "" {
		return Result{}, fmt.Errorf("%w: tenant_id, transfer_id, and spi_message_id are required", rail.ErrValidation)
	}

	current, err := s.store.GetTransfer(ctx, callback.TenantID, callback.TransferID)
	if err != nil {
		return Result{}, err
	}
	if current.Status.Terminal() {
		return Result{Transfer: current, IdempotentReplay: true}, nil
	}

	eventType := "pix_transfer_settled"
	if callback.Status == rail.SettlementRejected {
		eventType = "pix_transfer_rejected"
	}
	event, err := events.New(newID("evt"), eventType, current.TenantID, current.AccountID, current.ID, callback.CorrelationID, callback.ReceivedAt, map[string]any{
		"spi_message_id": callback.SPIMessageID,
		"status":         callback.Status,
		"code":           callback.Code,
	})
	if err != nil {
		return Result{}, err
	}
	audit := store.AuditRecord{
		TenantID:      current.TenantID,
		AccountID:     current.AccountID,
		TransferID:    current.ID,
		Action:        "spi_settlement_callback_recorded",
		CorrelationID: callback.CorrelationID,
		Metadata: map[string]string{
			"status": string(callback.Status),
			"code":   callback.Code,
		},
		CreatedAt: callback.ReceivedAt,
	}
	updated, err := s.store.UpdateSettlement(ctx, callback.TenantID, callback.TransferID, callback, []events.Event{event}, audit)
	if err != nil {
		return Result{}, err
	}
	return Result{Transfer: updated, Events: []events.Event{event}}, nil
}

func (s *Service) Outbox(ctx context.Context) []events.OutboxRecord {
	return s.store.Outbox(ctx)
}

func (s *Service) Audit(ctx context.Context) []store.AuditRecord {
	return s.store.Audit(ctx)
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
