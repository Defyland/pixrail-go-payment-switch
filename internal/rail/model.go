package rail

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type TransferStatus string

const (
	StatusAccepted TransferStatus = "accepted"
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
	ID                    string
	TenantID              string
	AccountID             string
	IdempotencyKey        string
	RequestHash           string
	CorrelationID         string
	EndToEndID            string
	AmountCents           int64
	Currency              string
	ReceiverKey           string
	ReceiverKeyType       DictKeyType
	ReceiverName          string
	ReceiverBank          string
	ReceiverRisk          int
	FraudScore            int
	FraudRules            []string
	Status                TransferStatus
	DecisionReason        string
	SPIMessageID          string
	SPIClaimToken         string
	SPIClaimedUntil       *time.Time
	SPISubmissionAttempts int
	SPILastError          string
	SettlementCode        string
	CreatedAt             time.Time
	UpdatedAt             time.Time
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
	CallbackHash  string
	CorrelationID string
	ReceivedAt    time.Time
}

type ReviewDecision string

const (
	ReviewApprove ReviewDecision = "approve"
	ReviewBlock   ReviewDecision = "block"
)

type ReviewDecisionRequest struct {
	TenantID      string
	TransferID    string
	Decision      ReviewDecision
	Reason        string
	CorrelationID string
	ReviewedAt    time.Time
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

func (r CreateTransferRequest) Fingerprint() string {
	currency := strings.TrimSpace(r.Currency)
	if currency == "" {
		currency = "BRL"
	}
	parts := []string{
		strings.TrimSpace(r.TenantID),
		strings.TrimSpace(r.AccountID),
		fmt.Sprintf("%d", r.AmountCents),
		currency,
		strings.TrimSpace(r.ReceiverKey),
		string(r.ReceiverKeyType),
		strings.TrimSpace(r.Description),
	}
	return hashParts(parts...)
}

func (c SettlementCallback) Fingerprint() string {
	return hashParts(
		strings.TrimSpace(c.TenantID),
		strings.TrimSpace(c.TransferID),
		strings.TrimSpace(c.SPIMessageID),
		string(c.Status),
		strings.TrimSpace(c.Code),
	)
}

func (s TransferStatus) Terminal() bool {
	return s == StatusBlocked || s == StatusSettled || s == StatusRejected
}

func (t Transfer) ClaimSPISubmission(claimToken string, claimUntil time.Time, now time.Time) (Transfer, error) {
	if strings.TrimSpace(claimToken) == "" || !claimUntil.After(now.UTC()) {
		return Transfer{}, fmt.Errorf("%w: valid spi claim token and expiry are required", ErrValidation)
	}
	if t.Status != StatusAccepted {
		return Transfer{}, fmt.Errorf("%w: transfer is not pending SPI submission", ErrConflict)
	}
	if t.SPIClaimedUntil != nil && t.SPIClaimedUntil.After(now.UTC()) {
		return Transfer{}, fmt.Errorf("%w: transfer already claimed for SPI submission", ErrConflict)
	}
	expiresAt := claimUntil.UTC()
	t.SPIClaimToken = claimToken
	t.SPIClaimedUntil = &expiresAt
	t.SPISubmissionAttempts++
	t.SPILastError = ""
	t.UpdatedAt = now.UTC()
	return t, nil
}

func (t Transfer) RecordSPISubmission(claimToken string, message SPIMessage) (Transfer, error) {
	if t.Status != StatusAccepted {
		return Transfer{}, fmt.Errorf("%w: transfer is not pending SPI submission", ErrConflict)
	}
	if t.SPIClaimToken == "" || t.SPIClaimToken != claimToken {
		return Transfer{}, fmt.Errorf("%w: transfer is not claimed by this SPI worker", ErrConflict)
	}
	if strings.TrimSpace(message.MessageID) == "" || strings.TrimSpace(message.EndToEndID) == "" {
		return Transfer{}, fmt.Errorf("%w: spi identifiers are required", ErrValidation)
	}
	t.Status = StatusApproved
	t.SPIMessageID = message.MessageID
	t.EndToEndID = message.EndToEndID
	t.SPIClaimToken = ""
	t.SPIClaimedUntil = nil
	t.SPILastError = ""
	t.UpdatedAt = message.SubmittedAt.UTC()
	return t, nil
}

func (t Transfer) ReleaseSPISubmission(claimToken string, lastError string, retryAt time.Time, now time.Time) (Transfer, error) {
	if t.Status == StatusApproved && t.SPIMessageID != "" {
		return t, nil
	}
	if t.SPIClaimToken == "" || t.SPIClaimToken != claimToken {
		return Transfer{}, fmt.Errorf("%w: transfer is not claimed by this SPI worker", ErrConflict)
	}
	availableAt := retryAt.UTC()
	t.SPIClaimToken = ""
	t.SPIClaimedUntil = &availableAt
	t.SPILastError = lastError
	t.UpdatedAt = now.UTC()
	return t, nil
}

func (t Transfer) RecordReviewDecision(status TransferStatus, reason string, reviewedAt time.Time) (Transfer, error) {
	if t.Status != StatusReview {
		return Transfer{}, fmt.Errorf("%w: transfer is not waiting for review", ErrConflict)
	}
	switch status {
	case StatusAccepted, StatusBlocked:
	default:
		return Transfer{}, fmt.Errorf("%w: review can only accept or block", ErrValidation)
	}
	t.Status = status
	if reason != "" {
		t.DecisionReason = reason
	}
	t.UpdatedAt = reviewedAt.UTC()
	return t, nil
}

func (t Transfer) RecordSettlement(callback SettlementCallback) (Transfer, error) {
	if t.SPIMessageID == "" || t.SPIMessageID != callback.SPIMessageID {
		return Transfer{}, fmt.Errorf("%w: spi_message_id mismatch", ErrConflict)
	}
	if t.Status.Terminal() {
		return Transfer{}, fmt.Errorf("%w: terminal transfer has no matching callback hash", ErrConflict)
	}
	if t.Status != StatusApproved {
		return Transfer{}, fmt.Errorf("%w: transfer is not approved for settlement callback", ErrConflict)
	}
	switch callback.Status {
	case SettlementAccepted:
		t.Status = StatusSettled
	case SettlementRejected:
		t.Status = StatusRejected
	default:
		return Transfer{}, fmt.Errorf("%w: unsupported settlement status", ErrValidation)
	}
	t.SettlementCode = callback.Code
	t.UpdatedAt = callback.ReceivedAt.UTC()
	return t, nil
}

func hashParts(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
