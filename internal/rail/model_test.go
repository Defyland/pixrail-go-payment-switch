package rail

import (
	"errors"
	"testing"
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
