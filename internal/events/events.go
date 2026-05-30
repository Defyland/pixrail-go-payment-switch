package events

import (
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion string          `json:"schema_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Producer      string          `json:"producer"`
	TenantID      string          `json:"tenant_id"`
	AccountID     string          `json:"account_id"`
	TransferID    string          `json:"pix_transfer_id"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload"`
}

const Producer = "pixrail.payment-switch"

func New(eventID, eventType, tenantID, accountID, transferID, correlationID string, occurredAt time.Time, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event payload: %w", err)
	}
	return Event{
		EventID:       eventID,
		EventType:     eventType,
		SchemaVersion: "1",
		OccurredAt:    occurredAt.UTC(),
		Producer:      Producer,
		TenantID:      tenantID,
		AccountID:     accountID,
		TransferID:    transferID,
		CorrelationID: correlationID,
		Payload:       raw,
	}, nil
}

type OutboxRecord struct {
	Sequence     int64
	Event        Event
	Published    bool
	Attempts     int
	LastError    string
	AvailableAt  time.Time
	DispatchedAt *time.Time
}
