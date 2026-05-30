package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
)

type MemoryStore struct {
	mu          sync.RWMutex
	transfers   map[string]rail.Transfer
	idempotency map[string]string
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
		events:      make([]events.OutboxRecord, 0, 128),
		audit:       make([]AuditRecord, 0, 128),
	}
}

func (s *MemoryStore) FindByIdempotency(_ context.Context, tenantID, key string) (rail.Transfer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.idempotency[tenantID+":"+key]
	if !ok {
		return rail.Transfer{}, false
	}
	transfer, ok := s.transfers[id]
	return transfer, ok
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

func (s *MemoryStore) UpdateSettlement(_ context.Context, tenantID string, transferID string, callback rail.SettlementCallback, outbox []events.Event, audit AuditRecord) (rail.Transfer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	transfer, ok := s.transfers[transferID]
	if !ok || transfer.TenantID != tenantID {
		return rail.Transfer{}, rail.ErrNotFound
	}
	if transfer.SPIMessageID == "" || transfer.SPIMessageID != callback.SPIMessageID {
		return rail.Transfer{}, fmt.Errorf("%w: spi_message_id mismatch", rail.ErrConflict)
	}
	if transfer.Status.Terminal() {
		return transfer, nil
	}
	switch callback.Status {
	case rail.SettlementAccepted:
		transfer.Status = rail.StatusSettled
	case rail.SettlementRejected:
		transfer.Status = rail.StatusRejected
	default:
		return rail.Transfer{}, fmt.Errorf("%w: unsupported settlement status", rail.ErrValidation)
	}
	transfer.SettlementCode = callback.Code
	transfer.UpdatedAt = callback.ReceivedAt.UTC()
	s.transfers[transfer.ID] = transfer
	s.appendEventsLocked(outbox)
	s.audit = append(s.audit, audit)
	return transfer, nil
}

func (s *MemoryStore) Outbox(_ context.Context) []events.OutboxRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]events.OutboxRecord, len(s.events))
	copy(records, s.events)
	return records
}

func (s *MemoryStore) Audit(_ context.Context) []AuditRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]AuditRecord, len(s.audit))
	copy(records, s.audit)
	return records
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
