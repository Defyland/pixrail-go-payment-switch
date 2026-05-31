package codec

import "time"

type EventType string

const (
	EventPixTransferRequested EventType = "pix_transfer_requested"
	EventPixTransferApproved  EventType = "pix_transfer_approved"
	EventPixTransferBlocked   EventType = "pix_transfer_blocked"
	EventSettlementAccepted   EventType = "pix_transfer_settled"
)

type PaymentEvent struct {
	EventID            string    `json:"event_id"`
	EventType          EventType `json:"event_type"`
	TenantID           string    `json:"tenant_id"`
	AccountID          string    `json:"account_id"`
	TransferID         string    `json:"pix_transfer_id"`
	CorrelationID      string    `json:"correlation_id"`
	OccurredAtUnixNano int64     `json:"occurred_at_unix_nano"`
	AmountCents        int64     `json:"amount_cents,omitempty"`
	Currency           string    `json:"currency,omitempty"`
	ReceiverKeyType    string    `json:"receiver_key_type,omitempty"`
	DecisionReason     string    `json:"decision_reason,omitempty"`
	SPIMessageID       string    `json:"spi_message_id,omitempty"`
	EndToEndID         string    `json:"end_to_end_id,omitempty"`
	FraudScore         int       `json:"fraud_score,omitempty"`
	FraudRules         []string  `json:"fraud_rules,omitempty"`
}

func SamplePixTransferRequested() PaymentEvent {
	return PaymentEvent{
		EventID:            "evt_20260531_requested_001",
		EventType:          EventPixTransferRequested,
		TenantID:           "tenant_demo",
		AccountID:          "acct_123",
		TransferID:         "pxt_20260531_001",
		CorrelationID:      "corr_20260531_001",
		OccurredAtUnixNano: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC).UnixNano(),
		AmountCents:        12345,
		Currency:           "BRL",
		ReceiverKeyType:    "EMAIL",
		FraudRules:         []string{},
	}
}

func SamplePixTransferApproved() PaymentEvent {
	event := SamplePixTransferRequested()
	event.EventID = "evt_20260531_approved_001"
	event.EventType = EventPixTransferApproved
	event.DecisionReason = "risk within payment-rail policy"
	event.SPIMessageID = "spi_6f0b1abf6b8c2f71"
	event.EndToEndID = "E202605311200006f0b1abf6b8c2f71"
	return event
}

func SamplePixTransferBlocked() PaymentEvent {
	event := SamplePixTransferRequested()
	event.EventID = "evt_20260531_blocked_001"
	event.EventType = EventPixTransferBlocked
	event.DecisionReason = "receiver risk over payment-rail policy"
	event.FraudScore = 96
	event.FraudRules = []string{"receiver_risk_high", "velocity_threshold"}
	return event
}
