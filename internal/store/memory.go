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
