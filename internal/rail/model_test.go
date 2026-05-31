package rail

import (
	"errors"
	"testing"
	"time"
)

func TestCreateTransferRequestValidateDefaultsBRL(t *testing.T) {
	req := CreateTransferRequest{
		TenantID:        "tenant_a",
		AccountID:       "acct_1",
		IdempotencyKey:  "idem",
		AmountCents:     100,
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: KeyEmail,
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request with default currency, got %v", err)
	}
}

func TestCreateTransferRequestValidateRejectsInvalidInput(t *testing.T) {
	req := CreateTransferRequest{Currency: "USD"}
	err := req.Validate()
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestTransferStatusTerminal(t *testing.T) {
	if !StatusSettled.Terminal() || !StatusBlocked.Terminal() || !StatusRejected.Terminal() {
		t.Fatal("expected settled, blocked, and rejected to be terminal")
	}
	if StatusAccepted.Terminal() || StatusApproved.Terminal() || StatusReview.Terminal() {
		t.Fatal("accepted, approved, and review are not terminal")
	}
}

func TestCreateTransferRequestFingerprintChangesWhenPayloadChanges(t *testing.T) {
	base := CreateTransferRequest{
		TenantID:        "tenant_a",
		AccountID:       "acct_1",
		IdempotencyKey:  "idem",
		AmountCents:     100,
		Currency:        "BRL",
		ReceiverKey:     "receiver@example.com",
		ReceiverKeyType: KeyEmail,
	}
	changed := base
	changed.AmountCents = 200
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("expected payload fingerprint to change when transfer payload changes")
	}
}

func TestSettlementCallbackFingerprintChangesWhenPayloadChanges(t *testing.T) {
	base := SettlementCallback{
		TenantID:     "tenant_a",
		TransferID:   "pxt_1",
		SPIMessageID: "spi_1",
		Status:       SettlementAccepted,
		Code:         "ACSC",
	}
	changed := base
	changed.Code = "RJCT"
	if base.Fingerprint() == changed.Fingerprint() {
		t.Fatal("expected callback fingerprint to change when callback payload changes")
	}
}

func TestTransferSPIStateMachine(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	transfer := Transfer{
		ID:        "pxt_1",
		Status:    StatusAccepted,
		CreatedAt: now,
		UpdatedAt: now,
	}

	claimed, err := transfer.ClaimSPISubmission("claim_1", now.Add(time.Minute), now)
	if err != nil {
		t.Fatalf("claim spi: %v", err)
	}
	if claimed.SPIClaimToken != "claim_1" || claimed.SPISubmissionAttempts != 1 {
		t.Fatalf("expected claim evidence, got %+v", claimed)
	}

	approved, err := claimed.RecordSPISubmission("claim_1", SPIMessage{
		MessageID:   "spi_1",
		EndToEndID:  "E123",
		SubmittedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("record spi: %v", err)
	}
	if approved.Status != StatusApproved || approved.SPIMessageID != "spi_1" || approved.SPIClaimToken != "" {
		t.Fatalf("expected approved transfer with cleared claim, got %+v", approved)
	}
}

func TestTransferRejectsInvalidSPITransitions(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	blocked := Transfer{ID: "pxt_1", Status: StatusBlocked}
	if _, err := blocked.ClaimSPISubmission("claim_1", now.Add(time.Minute), now); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected blocked transfer to reject spi claim, got %v", err)
	}

	accepted := Transfer{ID: "pxt_2", Status: StatusAccepted, SPIClaimToken: "claim_1"}
	if _, err := accepted.RecordSPISubmission("claim_2", SPIMessage{MessageID: "spi_1", EndToEndID: "E123", SubmittedAt: now}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected wrong claim token conflict, got %v", err)
	}
}

func TestTransferReviewStateMachine(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	review := Transfer{ID: "pxt_1", Status: StatusReview, DecisionReason: "manual review required"}

	accepted, err := review.RecordReviewDecision(StatusAccepted, "analyst approved", now)
	if err != nil {
		t.Fatalf("review approve: %v", err)
	}
	if accepted.Status != StatusAccepted || accepted.DecisionReason != "analyst approved" {
		t.Fatalf("expected review approval to move to accepted, got %+v", accepted)
	}

	if _, err := accepted.RecordReviewDecision(StatusBlocked, "late block", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected non-review transfer to reject review decision, got %v", err)
	}
}

func TestTransferSettlementStateMachine(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	approved := Transfer{ID: "pxt_1", Status: StatusApproved, SPIMessageID: "spi_1"}
	callback := SettlementCallback{
		TransferID:   approved.ID,
		SPIMessageID: "spi_1",
		Status:       SettlementAccepted,
		Code:         "ACSC",
		ReceivedAt:   now,
	}

	settled, err := approved.RecordSettlement(callback)
	if err != nil {
		t.Fatalf("settlement: %v", err)
	}
	if settled.Status != StatusSettled || settled.SettlementCode != "ACSC" {
		t.Fatalf("expected settled transfer, got %+v", settled)
	}

	callback.SPIMessageID = "spi_wrong"
	if _, err := approved.RecordSettlement(callback); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected spi message mismatch, got %v", err)
	}
}
