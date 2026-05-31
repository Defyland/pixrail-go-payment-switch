package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

type MemoryStore struct {
	mu          sync.RWMutex
	transfers   map[string]rail.Transfer
	idempotency map[string]string
	callbacks   map[string]string
	events      []events.OutboxRecord
	audit       []AuditRecord
	nextSeq     int64
}

type AuditRecord struct {
	TenantID      string
	AccountID     string
	TransferID    string
	Action        string
	CorrelationID string
	Metadata      map[string]string
	CreatedAt     time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		transfers:   make(map[string]rail.Transfer),
		idempotency: make(map[string]string),
		callbacks:   make(map[string]string),
		events:      make([]events.OutboxRecord, 0, 128),
		audit:       make([]AuditRecord, 0, 128),
	}
}

func (s *MemoryStore) Health(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *MemoryStore) FindByIdempotency(ctx context.Context, tenantID, key string) (rail.Transfer, bool, error) {
	if err := s.Health(ctx); err != nil {
		return rail.Transfer{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.idempotency[tenantID+":"+key]
	if !ok {
		return rail.Transfer{}, false, nil
	}
	transfer, ok := s.transfers[id]
	return transfer, ok, nil
}

func (s *MemoryStore) InsertTransfer(_ context.Context, transfer rail.Transfer, outbox []events.Event, audit []AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idem := transfer.TenantID + ":" + transfer.IdempotencyKey
	if existingID, ok := s.idempotency[idem]; ok && existingID != transfer.ID {
		return fmt.Errorf("%w: idempotency key already used", rail.ErrConflict)
	}
	if _, ok := s.transfers[transfer.ID]; ok {
		return fmt.Errorf("%w: transfer already exists", rail.ErrConflict)
	}
	s.transfers[transfer.ID] = transfer
	s.idempotency[idem] = transfer.ID
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit...)
	return nil
}

func (s *MemoryStore) GetTransfer(_ context.Context, tenantID, transferID string) (rail.Transfer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, rail.ErrNotFound
	}
	return transfer, nil
}

func (s *MemoryStore) RecordSPISubmission(_ context.Context, tenantID string, transferID string, message rail.SPIMessage, outbox []events.Event, audit AuditRecord) (rail.Transfer, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID == message.MessageID {
		return transfer, true, nil
	}
	if transfer.Status != rail.StatusAccepted {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not pending SPI submission", rail.ErrConflict)
	}
	if message.MessageID == "" || message.EndToEndID == "" {
		return rail.Transfer{}, false, fmt.Errorf("%w: spi identifiers are required", rail.ErrValidation)
	}
	transfer.Status = rail.StatusApproved
	transfer.SPIMessageID = message.MessageID
	transfer.EndToEndID = message.EndToEndID
	transfer.UpdatedAt = message.SubmittedAt.UTC()
	s.transfers[transfer.ID] = transfer
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, false, nil
}

func (s *MemoryStore) RecordReviewDecision(_ context.Context, tenantID string, transferID string, status rail.TransferStatus, reason string, outbox []events.Event, audit AuditRecord) (rail.Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, rail.ErrNotFound
	}
	if transfer.Status != rail.StatusReview {
		return rail.Transfer{}, fmt.Errorf("%w: transfer is not waiting for review", rail.ErrConflict)
	}
	switch status {
	case rail.StatusAccepted, rail.StatusBlocked:
	default:
		return rail.Transfer{}, fmt.Errorf("%w: review can only accept or block", rail.ErrValidation)
	}
	transfer.Status = status
	if reason != "" {
		transfer.DecisionReason = reason
	}
	transfer.UpdatedAt = audit.CreatedAt.UTC()
	s.transfers[transfer.ID] = transfer
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, nil
}

func (s *MemoryStore) UpdateSettlement(_ context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit AuditRecord) (rail.Transfer, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if transfer.SPIMessageID == "" || transfer.SPIMessageID != callback.SPIMessageID {
		return rail.Transfer{}, false, fmt.Errorf("%w: spi_message_id mismatch", rail.ErrConflict)
	}
	callbackHash := callback.CallbackHash
	if callbackHash == "" {
		callbackHash = callback.Fingerprint()
	}
	callbackKey := tenantID + ":" + callback.SPIMessageID
	if existingHash, ok := s.callbacks[callbackKey]; ok {
		if existingHash == callbackHash {
			return transfer, true, nil
		}
		return rail.Transfer{}, false, fmt.Errorf("%w: conflicting settlement callback for terminal transfer", rail.ErrConflict)
	}
	if transfer.Status.Terminal() {
		return rail.Transfer{}, false, fmt.Errorf("%w: terminal transfer has no matching callback hash", rail.ErrConflict)
	}
	if transfer.Status != rail.StatusApproved {
		return rail.Transfer{}, false, fmt.Errorf("%w: transfer is not approved for settlement callback", rail.ErrConflict)
	}
	switch callback.Status {
	case rail.SettlementAccepted:
		transfer.Status = rail.StatusSettled
	case rail.SettlementRejected:
		transfer.Status = rail.StatusRejected
	default:
		return rail.Transfer{}, false, fmt.Errorf("%w: unsupported settlement status", rail.ErrValidation)
	}
	transfer.SettlementCode = callback.Code
	transfer.UpdatedAt = callback.ReceivedAt.UTC()
	s.transfers[transfer.ID] = transfer
	s.callbacks[callbackKey] = callbackHash
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, false, nil
}

func (s *MemoryStore) PendingSPISubmissions(ctx context.Context, limit int) ([]rail.Transfer, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	transfers := make([]rail.Transfer, 0, limit)
	for _, transfer := range s.transfers {
		if transfer.Status != rail.StatusAccepted || transfer.SPIMessageID != "" {
			continue
		}
		transfers = append(transfers, transfer)
	}
	sort.Slice(transfers, func(i, j int) bool {
		return transfers[i].CreatedAt.Before(transfers[j].CreatedAt)
	})
	if len(transfers) > limit {
		transfers = transfers[:limit]
	}
	return transfers, nil
}

func (s *MemoryStore) Outbox(ctx context.Context) ([]events.OutboxRecord, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]events.OutboxRecord, len(s.events))
	copy(records, s.events)
	return records, nil
}

func (s *MemoryStore) PendingOutbox(ctx context.Context, limit int) ([]events.OutboxRecord, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := make([]events.OutboxRecord, 0, limit)
	for _, record := range s.events {
		if record.Published || record.AvailableAt.After(now) {
			continue
		}
		records = append(records, record)
		if len(records) == limit {
			break
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Sequence < records[j].Sequence
	})
	return records, nil
}

func (s *MemoryStore) MarkOutboxPublished(ctx context.Context, sequence int64, dispatchedAt time.Time) error {
	if err := s.Health(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.events {
		if s.events[index].Sequence != sequence {
			continue
		}
		s.events[index].Published = true
		s.events[index].LastError = ""
		dispatched := dispatchedAt.UTC()
		s.events[index].DispatchedAt = &dispatched
		return nil
	}
	return fmt.Errorf("%w: outbox sequence not found", rail.ErrNotFound)
}

func (s *MemoryStore) MarkOutboxFailed(ctx context.Context, sequence int64, lastError string, retryAt time.Time) error {
	if err := s.Health(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.events {
		if s.events[index].Sequence != sequence {
			continue
		}
		s.events[index].Attempts++
		s.events[index].LastError = lastError
		s.events[index].AvailableAt = retryAt.UTC()
		return nil
	}
	return fmt.Errorf("%w: outbox sequence not found", rail.ErrNotFound)
}

func (s *MemoryStore) Audit(ctx context.Context) ([]AuditRecord, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]AuditRecord, len(s.audit))
	copy(records, s.audit)
	return records, nil
}

func (s *MemoryStore) appendEventsLocked(outbox []events.Event) {
	for _, event := range outbox {
		s.nextSeq++
		s.events = append(s.events, events.OutboxRecord{
			Sequence:    s.nextSeq,
			Event:       event,
			Published:   false,
			AvailableAt: event.OccurredAt,
		})
	}
}
