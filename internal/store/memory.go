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
	audit       []rail.AuditRecord
	nextSeq     int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		transfers:   make(map[string]rail.Transfer),
		idempotency: make(map[string]string),
		callbacks:   make(map[string]string),
		events:      make([]events.OutboxRecord, 0, 128),
		audit:       make([]rail.AuditRecord, 0, 128),
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

func (s *MemoryStore) InsertTransfer(_ context.Context, transfer rail.Transfer, outbox []events.Event, audit []rail.AuditRecord) error {
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

func (s *MemoryStore) ClaimSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, claimUntil time.Time) (rail.Transfer, bool, error) {
	if err := s.Health(ctx); err != nil {
		return rail.Transfer{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID != "" {
		return transfer, true, nil
	}
	transfer, err := transfer.ClaimSPISubmission(claimToken, claimUntil, now)
	if err != nil {
		return rail.Transfer{}, false, err
	}
	s.transfers[transfer.ID] = transfer
	return transfer, false, nil
}

func (s *MemoryStore) RecordSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, message rail.SPIMessage, outbox []events.Event, audit rail.AuditRecord) (rail.Transfer, bool, error) {
	if err := s.Health(ctx); err != nil {
		return rail.Transfer{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, false, rail.ErrNotFound
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID == message.MessageID {
		return transfer, true, nil
	}
	transfer, err := transfer.RecordSPISubmission(claimToken, message)
	if err != nil {
		return rail.Transfer{}, false, err
	}
	s.transfers[transfer.ID] = transfer
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, false, nil
}

func (s *MemoryStore) ReleaseSPISubmission(ctx context.Context, tenantID string, transferID string, claimToken string, lastError string, retryAt time.Time) error {
	if err := s.Health(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.ErrNotFound
	}
	if transfer.Status == rail.StatusApproved && transfer.SPIMessageID != "" {
		return nil
	}
	transfer, err := transfer.ReleaseSPISubmission(claimToken, lastError, retryAt, time.Now().UTC())
	if err != nil {
		return err
	}
	s.transfers[transfer.ID] = transfer
	return nil
}

func (s *MemoryStore) RecordReviewDecision(_ context.Context, tenantID string, transferID string, status rail.TransferStatus, reason string, outbox []events.Event, audit rail.AuditRecord) (rail.Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, rail.ErrNotFound
	}
	transfer, err := transfer.RecordReviewDecision(status, reason, audit.CreatedAt)
	if err != nil {
		return rail.Transfer{}, err
	}
	s.transfers[transfer.ID] = transfer
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, nil
}

func (s *MemoryStore) UpdateSettlement(_ context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit rail.AuditRecord) (rail.Transfer, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, false, rail.ErrNotFound
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
	transfer, err := transfer.RecordSettlement(callback)
	if err != nil {
		return rail.Transfer{}, false, err
	}
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
	now := time.Now().UTC()
	s.mu.RLock()
	defer s.mu.RUnlock()
	transfers := make([]rail.Transfer, 0, limit)
	for _, transfer := range s.transfers {
		if transfer.Status != rail.StatusAccepted || transfer.SPIMessageID != "" {
			continue
		}
		if transfer.SPIClaimedUntil != nil && transfer.SPIClaimedUntil.After(now) {
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
		if record.ClaimedUntil != nil && record.ClaimedUntil.After(now) {
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

func (s *MemoryStore) ClaimPendingOutbox(ctx context.Context, limit int, claimToken string, claimUntil time.Time) ([]events.OutboxRecord, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if claimToken == "" || !claimUntil.After(time.Now().UTC()) {
		return nil, fmt.Errorf("%w: valid outbox claim token and expiry are required", rail.ErrValidation)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]events.OutboxRecord, 0, limit)
	for index := range s.events {
		record := s.events[index]
		if record.Published || record.AvailableAt.After(now) {
			continue
		}
		if record.ClaimedUntil != nil && record.ClaimedUntil.After(now) {
			continue
		}
		claimExpiry := claimUntil.UTC()
		s.events[index].ClaimToken = claimToken
		s.events[index].ClaimedUntil = &claimExpiry
		records = append(records, s.events[index])
		if len(records) == limit {
			break
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Sequence < records[j].Sequence
	})
	return records, nil
}

func (s *MemoryStore) MarkOutboxPublished(ctx context.Context, sequence int64, claimToken string, dispatchedAt time.Time) error {
	if err := s.Health(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.events {
		if s.events[index].Sequence != sequence {
			continue
		}
		if s.events[index].ClaimToken == "" || s.events[index].ClaimToken != claimToken {
			return fmt.Errorf("%w: outbox record is not claimed by this worker", rail.ErrConflict)
		}
		s.events[index].Published = true
		s.events[index].LastError = ""
		s.events[index].ClaimToken = ""
		s.events[index].ClaimedUntil = nil
		dispatched := dispatchedAt.UTC()
		s.events[index].DispatchedAt = &dispatched
		return nil
	}
	return fmt.Errorf("%w: outbox sequence not found", rail.ErrNotFound)
}

func (s *MemoryStore) MarkOutboxFailed(ctx context.Context, sequence int64, claimToken string, lastError string, retryAt time.Time) error {
	if err := s.Health(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.events {
		if s.events[index].Sequence != sequence {
			continue
		}
		if s.events[index].ClaimToken == "" || s.events[index].ClaimToken != claimToken {
			return fmt.Errorf("%w: outbox record is not claimed by this worker", rail.ErrConflict)
		}
		s.events[index].Attempts++
		s.events[index].LastError = lastError
		s.events[index].AvailableAt = retryAt.UTC()
		s.events[index].ClaimToken = ""
		s.events[index].ClaimedUntil = nil
		return nil
	}
	return fmt.Errorf("%w: outbox sequence not found", rail.ErrNotFound)
}

func (s *MemoryStore) Audit(ctx context.Context) ([]rail.AuditRecord, error) {
	if err := s.Health(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]rail.AuditRecord, len(s.audit))
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
