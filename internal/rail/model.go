package rail

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type TransferStatus string

const (
	StatusApproved TransferStatus = "approved"
	StatusBlocked  TransferStatus = "blocked"
	StatusReview   TransferStatus = "review"
	StatusSettled  TransferStatus = "settled"
	StatusRejected TransferStatus = "rejected"
)

type DictKeyType string

const (
	KeyCPF   DictKeyType = "CPF"
	KeyCNPJ  DictKeyType = "CNPJ"
	KeyEmail DictKeyType = "EMAIL"
	KeyPhone DictKeyType = "PHONE"
	KeyEVP   DictKeyType = "EVP"
)

type CreateTransferRequest struct {
	TenantID        string
	AccountID       string
	IdempotencyKey  string
	CorrelationID   string
	AmountCents     int64
	Currency        string
	ReceiverKey     string
	ReceiverKeyType DictKeyType
	Description     string
	RequestedAt     time.Time
}

type Transfer struct {
	ID              string
	TenantID        string
	AccountID       string
	IdempotencyKey  string
	CorrelationID   string
	EndToEndID      string
	AmountCents     int64
	Currency        string
	ReceiverKey     string
	ReceiverKeyType DictKeyType
	ReceiverName    string
	ReceiverBank    string
	ReceiverRisk    int
	FraudScore      int
	FraudRules      []string
	Status          TransferStatus
	DecisionReason  string
	SPIMessageID    string
	SettlementCode  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DictEntry struct {
	Key         string
	KeyType     DictKeyType
	ReceiverID  string
	Name        string
	BankISPB    string
	AccountHash string
	RiskSignal  int
	ResolvedAt  time.Time
}

type FraudDecision struct {
	Score  int
	Status TransferStatus
	Rules  []string
	Reason string
}

type SPIMessage struct {
	MessageID   string
	EndToEndID  string
	TransferID  string
	SubmittedAt time.Time
}

type SettlementStatus string

const (
	SettlementAccepted SettlementStatus = "accepted"
	SettlementRejected SettlementStatus = "rejected"
)

type SettlementCallback struct {
	TenantID      string
	TransferID    string
	SPIMessageID  string
	Status        SettlementStatus
	Code          string
	CorrelationID string
	ReceivedAt    time.Time
}

var (
	ErrValidation       = errors.New("validation failed")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrRateLimited      = errors.New("rate limited")
	ErrDependencyFailed = errors.New("dependency failed")
)

func (r CreateTransferRequest) Validate() error {
	var problems []string
	if strings.TrimSpace(r.TenantID) == "" {
		problems = append(problems, "tenant_id is required")
	}
	if strings.TrimSpace(r.AccountID) == "" {
		problems = append(problems, "account_id is required")
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		problems = append(problems, "idempotency key is required")
	}
	if r.AmountCents <= 0 {
		problems = append(problems, "amount_cents must be greater than zero")
	}
	if r.AmountCents > 2_000_000 {
		problems = append(problems, "amount_cents exceeds per-transfer limit")
	}
	if r.Currency == "" {
		r.Currency = "BRL"
	}
	if r.Currency != "BRL" {
		problems = append(problems, "currency must be BRL")
	}
	if strings.TrimSpace(r.ReceiverKey) == "" {
		problems = append(problems, "receiver_key is required")
	}
	switch r.ReceiverKeyType {
	case KeyCPF, KeyCNPJ, KeyEmail, KeyPhone, KeyEVP:
	default:
		problems = append(problems, "receiver_key_type is invalid")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(problems, "; "))
	}
	return nil
}

func (s TransferStatus) Terminal() bool {
	return s == StatusBlocked || s == StatusSettled || s == StatusRejected
}
